/*******************************************************************************
 * Copyright (c) 2023 Genome Research Ltd.
 *
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
 * CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
 * TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 ******************************************************************************/

package build

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/s3"
	"github.com/wtsi-hgi/go-softpack-builder/wr"
)

const (
	singularityDefBasename = "singularity.def"
	exesBasename           = "executables"
	softpackYaml           = "softpack.yml"
	spackLock              = "spack.lock"
	builderOut             = "builder.out"
	moduleForCoreBasename  = "module"
)

//go:embed singularity.tmpl
var singularityTmplStr string
var singularityTmpl *template.Template //nolint:gochecknoglobals

//go:embed softpack.tmpl
var softpackTmplStr string
var softpackTmpl *template.Template //nolint:gochecknoglobals

func init() { //nolint:gochecknoinits
	singularityTmpl = template.Must(template.New("").Parse(singularityTmplStr))
	softpackTmpl = template.Must(template.New("").Parse(softpackTmplStr))
}

type Error string

func (e Error) Error() string { return string(e) }

const ErrInvalidJSON = Error("invalid spack lock JSON")

type Package struct {
	Name    string
	Version string
}

type Definition struct {
	EnvironmentPath    string
	EnvironmentName    string
	EnvironmentVersion string
	Description        string
	Packages           []Package
}

type Builder struct {
	config *config.Config
	s3     interface {
		UploadData(data io.Reader, dest string) error
		DownloadFile(source, dest string) error
		OpenFile(source string) (io.ReadCloser, error)
	}
	runner interface {
		Run(deployment string) error
	}
}

// New takes the s3 build cache URL, the repo and checkout reference of your
// custom spack repo, and returns a Builder.
func New(config *config.Config) (*Builder, error) {
	s3helper, err := s3.New(config.S3.BuildBase)
	if err != nil {
		return nil, err
	}

	return &Builder{
		config: config,
		s3:     s3helper,
		runner: wr.New(config.WRDeployment),
	}, nil
}

type templateVars struct {
	BuildCache string
	RepoURL    string
	RepoRef    string
	Packages   []Package
}

func (b *Builder) GenerateSingularityDef(def *Definition) (string, error) {
	var w strings.Builder
	err := singularityTmpl.Execute(&w, &templateVars{
		BuildCache: b.config.S3.BinaryCache,
		RepoURL:    b.config.CustomSpackRepo.URL,
		RepoRef:    b.config.CustomSpackRepo.Ref,
		Packages:   def.Packages,
	})

	return w.String(), err
}

func (b *Builder) Build(def *Definition) error {
	singDef, err := b.GenerateSingularityDef(def)
	if err != nil {
		return err
	}

	s3Path := filepath.Join(
		def.EnvironmentPath, def.EnvironmentName,
		def.EnvironmentVersion,
	)

	singDefUploadPath := filepath.Join(s3Path, singularityDefBasename)

	if err = b.s3.UploadData(strings.NewReader(singDef), singDefUploadPath); err != nil {
		return err
	}

	singDefParentPath := filepath.Join(b.config.S3.BuildBase, s3Path)

	wrInput, err := wr.SingularityBuildInS3WRInput(singDefParentPath)
	if err != nil {
		return err
	}

	go func() {
		if errb := b.asyncBuild(def, wrInput, s3Path, singDef); errb != nil {
			slog.Error("Async part of build failed", "err", errb.Error(), "s3Path", singDefParentPath)
		}
	}()

	return nil
}

func (b *Builder) asyncBuild(def *Definition, wrInput, s3Path, singDef string) error {
	err := b.runner.Run(wrInput)
	if err != nil {
		// TODO: upload logData
		return err
	}

	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir)

	imagePath := filepath.Join(tmpDir, imageBasename)

	err = b.s3.DownloadFile(filepath.Join(s3Path, imageBasename), imagePath)
	if err != nil {
		return err
	}

	moduleFileData := def.ToModule(b.config.Module.InstallDir, b.config.Module.Dependencies)
	// TODO: usageFileData := def.ModuleUsage(b.config.Module.LoadPath)

	imageFile, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer imageFile.Close()

	logData, err := b.s3.OpenFile(filepath.Join(s3Path, builderOut))
	if err != nil {
		return err
	}

	lockFile, err := b.s3.OpenFile(filepath.Join(s3Path, spackLock))
	if err != nil {
		return err
	}

	lockData, err := io.ReadAll(lockFile)
	if err != nil {
		return err
	}

	concreteSpackYAMLFile, err := SpackLockToSoftPackYML(lockData, def.Description)
	if err != nil {
		return err
	}

	err = b.AddArtifactsToRepo(
		imageFile,
		bytes.NewReader(lockData),
		concreteSpackYAMLFile,
		strings.NewReader(singDef),
		logData,
		strings.NewReader(moduleFileData),
		filepath.Join(def.EnvironmentPath, def.EnvironmentName+"-"+def.EnvironmentVersion),
	)
	if err != nil {
		return err
	}

	exeData, err := b.s3.OpenFile(filepath.Join(s3Path, exesBasename))
	if err != nil {
		return err
	}

	exes, err := executablesFileToExes(exeData)
	if err != nil {
		return err
	}

	imageFile, err = os.Open(imagePath)
	if err != nil {
		return err
	}
	defer imageFile.Close()

	return InstallModule(b.config.Module.InstallDir, def,
		strings.NewReader(moduleFileData), imageFile, exes, b.config.Module.WrapperScript)
}

func executablesFileToExes(data io.Reader) ([]string, error) {
	buf, err := io.ReadAll(data)
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.TrimSpace(string(buf)), "\n"), nil
}

type ConcreteSpec struct {
	Name, Version string
}

type SpackLock struct {
	Roots []struct {
		Hash, Spec string
	}
	ConcreteSpecs map[string]ConcreteSpec `json:"concrete_specs"`
}

func SpackLockToSoftPackYML(data []byte, desc string) (io.Reader, error) {
	var sl SpackLock

	if err := json.Unmarshal(data, &sl); err != nil {
		return nil, err
	}

	var concreteSpecs []ConcreteSpec

	for _, root := range sl.Roots {
		concrete, ok := sl.ConcreteSpecs[root.Hash]
		if !ok {
			return nil, ErrInvalidJSON
		}

		concreteSpecs = append(concreteSpecs, concrete)
	}

	var sb strings.Builder

	if err := softpackTmpl.Execute(&sb, struct {
		Description []string
		Packages    []ConcreteSpec
	}{
		Description: strings.Split(desc, "\n"),
		Packages:    concreteSpecs,
	}); err != nil {
		return nil, err
	}

	return strings.NewReader(sb.String()), nil
}

func (b *Builder) AddArtifactsToRepo(image, lock, softpackYML, singDef, log, module io.Reader, envPath string) error {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	errCh := make(chan error, 1)

	go func() {
		_, err := http.Post(b.config.CoreURL+"?"+url.QueryEscape(envPath), writer.FormDataContentType(), pr)
		errCh <- err
	}()

	for name, r := range map[string]io.Reader{
		imageBasename:          image,
		spackLock:              lock,
		softpackYaml:           softpackYML,
		singularityDefBasename: singDef,
		builderOut:             log,
		moduleForCoreBasename:  module,
	} {
		part, err := writer.CreateFormFile("file", name)
		if err != nil {
			return err
		}

		_, err = io.Copy(part, r)
		if err != nil {
			return err
		}
	}

	err := writer.Close()
	if err != nil {
		return err
	}

	err = pw.Close()
	if err != nil {
		return err
	}

	return <-errCh
}
