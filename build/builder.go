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
	"context"
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
	"sync"
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
	usageBasename          = "README.md"
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

const (
	ErrInvalidJSON         = Error("invalid spack lock JSON")
	ErrEnvironmentBuilding = Error("build already running for environment")
)

// Package describes the name and optional version of a spack package.
type Package struct {
	Name    string
	Version string
}

// Definition describes the environment a user wanted to create, which
// comprises a EnvironmentPath such as "users/username", and EnvironmentName
// such as "mainpackage", and EnvironmentVersion, such as "1". The given
// Packages will be installed for this Environment, and the Description will
// become the help text for making use of the Packages.
type Definition struct {
	EnvironmentPath    string
	EnvironmentName    string
	EnvironmentVersion string
	Description        string
	Packages           []Package
}

// FullEnvironmentPath returns the complete environment path: the location under
// environments in the environments git repository that the artefacts will be
// stored.
func (d *Definition) FullEnvironmentPath() string {
	return filepath.Join(d.EnvironmentPath, d.EnvironmentName+"-"+d.EnvironmentVersion)
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

	mu                  sync.Mutex
	runningEnvironments map[string]bool
}

// New takes the s3 build cache URL, the repo and checkout reference of your
// custom spack repo, and returns a Builder.
func New(config *config.Config) (*Builder, error) {
	s3helper, err := s3.New(config.S3.BuildBase)
	if err != nil {
		return nil, err
	}

	return &Builder{
		config:              config,
		s3:                  s3helper,
		runner:              wr.New(config.WRDeployment),
		runningEnvironments: make(map[string]bool),
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

func (b *Builder) Build(def *Definition) (err error) {
	var fn func()

	fn, err = b.protectEnvironment(def.FullEnvironmentPath(), &err)
	if err != nil {
		return err
	}

	defer fn()

	var singDef, wrInput string

	s3Path := filepath.Join(def.EnvironmentPath, def.EnvironmentName, def.EnvironmentVersion)

	if singDef, err = b.generateAndUploadSingularityDef(def, s3Path); err != nil {
		return err
	}

	singDefParentPath := filepath.Join(b.config.S3.BuildBase, s3Path)

	wrInput, err = wr.SingularityBuildInS3WRInput(singDefParentPath)
	if err != nil {
		return err
	}

	go b.startBuild(def, wrInput, s3Path, singDef, singDefParentPath)

	return nil
}

func (b *Builder) protectEnvironment(envPath string, err *error) (func(), error) {
	b.mu.Lock()

	if b.runningEnvironments[envPath] {
		b.mu.Unlock()

		return nil, ErrEnvironmentBuilding
	}

	b.runningEnvironments[envPath] = true

	b.mu.Unlock()

	return func() {
		if *err != nil {
			b.unprotectEnvironment(envPath)
		}
	}, nil
}

func (b *Builder) unprotectEnvironment(envPath string) {
	b.mu.Lock()
	delete(b.runningEnvironments, envPath)
	b.mu.Unlock()
}

func (b *Builder) generateAndUploadSingularityDef(def *Definition, s3Path string) (string, error) {
	singDef, err := b.GenerateSingularityDef(def)
	if err != nil {
		return "", err
	}

	singDefUploadPath := filepath.Join(s3Path, singularityDefBasename)

	err = b.s3.UploadData(strings.NewReader(singDef), singDefUploadPath)

	return singDef, err
}

func (b *Builder) startBuild(def *Definition, wrInput, s3Path, singDef, singDefParentPath string) {
	defer b.unprotectEnvironment(def.FullEnvironmentPath())

	if err := b.asyncBuild(def, wrInput, s3Path, singDef); err != nil {
		slog.Error("Async part of build failed", "err", err.Error(), "s3Path", singDefParentPath)
	}
}

func (b *Builder) asyncBuild(def *Definition, wrInput, s3Path, singDef string) error {
	err := b.runner.Run(wrInput)
	if err != nil {
		b.AddLogToRepo(s3Path, def.FullEnvironmentPath())

		return err
	}

	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir)

	imagePath := filepath.Join(tmpDir, imageBasename)

	moduleFileData := def.ToModule(b.config.Module.InstallDir, b.config.Module.Dependencies)

	if err = b.prepareArtifactsFromS3AndSendToCore(def, s3Path, imagePath, moduleFileData, singDef); err != nil {
		return err
	}

	return b.prepareAndInstallArtifacts(def, s3Path, imagePath, moduleFileData)
}

func (b *Builder) prepareArtifactsFromS3AndSendToCore(def *Definition, s3Path,
	imagePath, moduleFileData, singDef string) error {
	logData, lockData, imageData, err := b.getArtifactDataFromS3(s3Path, imagePath)
	if err != nil {
		return err
	}

	defer imageData.Close()

	concreteSpackYAMLFile, err := SpackLockToSoftPackYML(lockData, def.Description)
	if err != nil {
		return err
	}

	return b.AddArtifactsToRepo(
		map[string]io.Reader{
			imageBasename:          imageData,
			spackLock:              bytes.NewReader(lockData),
			softpackYaml:           concreteSpackYAMLFile,
			singularityDefBasename: strings.NewReader(singDef),
			builderOut:             logData,
			moduleForCoreBasename:  strings.NewReader(moduleFileData),
			usageBasename:          strings.NewReader(def.ModuleUsage(b.config.Module.LoadPath)),
		},
		def.FullEnvironmentPath(),
	)
}

func (b *Builder) AddLogToRepo(s3Path, environmentPath string) {
	log, err := b.s3.OpenFile(filepath.Join(s3Path, builderOut))
	if err != nil {
		slog.Error("error getting build log file", "err", err)
	}

	if err := b.AddArtifactsToRepo(map[string]io.Reader{
		builderOut: log,
	}, environmentPath); err != nil {
		slog.Error("error sending build log file to core", "err", err)
	}
}

func (b *Builder) getArtifactDataFromS3(s3Path, imagePath string) (io.Reader, []byte, io.ReadCloser, error) {
	if err := b.s3.DownloadFile(filepath.Join(s3Path, imageBasename), imagePath); err != nil {
		return nil, nil, nil, err
	}

	logData, err := b.s3.OpenFile(filepath.Join(s3Path, builderOut))
	if err != nil {
		return nil, nil, nil, err
	}

	lockFile, err := b.s3.OpenFile(filepath.Join(s3Path, spackLock))
	if err != nil {
		return nil, nil, nil, err
	}

	lockData, err := io.ReadAll(lockFile)
	if err != nil {
		return nil, nil, nil, err
	}

	imageFile, err := os.Open(imagePath)
	if err != nil {
		return nil, nil, nil, err
	}

	return logData, lockData, imageFile, nil
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

	concreteSpecs := make([]ConcreteSpec, len(sl.Roots))

	for i, root := range sl.Roots {
		concrete, ok := sl.ConcreteSpecs[root.Hash]
		if !ok {
			return nil, ErrInvalidJSON
		}

		concreteSpecs[i] = concrete
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

func (b *Builder) AddArtifactsToRepo(artifacts map[string]io.Reader, envPath string) error { //nolint:misspell
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	errCh := make(chan error, 1)

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	go func() {
		errCh <- sendFormFiles(artifacts, writer, pw) //nolint:misspell
	}()

	defer pw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.config.CoreURL+"?"+url.QueryEscape(envPath), pr)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", writer.FormDataContentType())

	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	return <-errCh
}

func sendFormFiles(artifacts map[string]io.Reader, //nolint:misspell
	writer *multipart.Writer, writerInput io.Closer) error {
	for name, r := range artifacts { //nolint:misspell
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

	return writerInput.Close()
}

func (b *Builder) prepareAndInstallArtifacts(def *Definition, s3Path, imagePath, moduleFileData string) error {
	exeData, err := b.s3.OpenFile(filepath.Join(s3Path, exesBasename))
	if err != nil {
		return err
	}

	exes, err := executablesFileToExes(exeData)
	if err != nil {
		return err
	}

	imageFile, err := os.Open(imagePath)
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
