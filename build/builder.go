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

	ErrInvalidEnvPath = Error("invalid environment path")
	ErrInvalidVersion = Error("environment version required")
	ErrNoPackages     = Error("packages required")
	ErrNoPackageName  = Error("package names required")
)

// Package describes the name and optional version of a spack package.
type Package struct {
	Name    string
	Version string
}

// Validate returns an error if Name isn't set.
func (p *Package) Validate() error {
	if p.Name == "" {
		return ErrNoPackageName
	}

	return nil
}

type Packages []Package

// Validate returns an error if p is zero length, or any of its Packages are
// invalid.
func (p Packages) Validate() error {
	if len(p) == 0 {
		return ErrNoPackages
	}

	for _, pkg := range p {
		if pkg.Name == "" {
			return ErrNoPackageName
		}
	}

	return nil
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
	Packages           Packages
}

// FullEnvironmentPath returns the complete environment path: the location under
// environments in the environments git repository that the artefacts will be
// stored.
func (d *Definition) FullEnvironmentPath() string {
	return filepath.Join(d.EnvironmentPath, d.EnvironmentName+"-"+d.EnvironmentVersion)
}

// Interpreters returns interpreter executable names required by
// interpreter-specific packages.
func (d *Definition) Interpreters() []string {
	var hasR, hasPython bool

	for _, pkg := range d.Packages {
		if strings.HasPrefix(pkg.Name, "r-") {
			hasR = true
		}

		if strings.HasPrefix(pkg.Name, "py-") {
			hasPython = true
		}
	}

	var interpreters []string

	if hasR {
		interpreters = append(interpreters, "R", "Rscript")
	}

	if hasPython {
		interpreters = append(interpreters, "python")
	}

	return interpreters
}

// Validate returns an error if the Path is invalid, if Version isn't set, if
// there are no packages defined, or if any package has no name.
func (d *Definition) Validate() error {
	epParts := strings.Split(d.EnvironmentPath, "/")
	if len(epParts) != 2 && !(epParts[0] == "groups" || epParts[0] == "users") {
		return ErrInvalidEnvPath
	}

	if d.EnvironmentVersion == "" {
		return ErrInvalidVersion
	}

	return d.Packages.Validate()
}

// Builder lets you do builds given config, S3 and a wr runner.
type Builder struct {
	config *config.Config
	s3     interface {
		UploadData(data io.Reader, dest string) error
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
	S3BinaryCache    string
	RepoURL          string
	RepoRef          string
	SpackBinaryCache string
	BuildImage       string
	FinalImage       string
	ExtraExes        []string
	Packages         []Package
}

// Build uploads a singularity.def generated by GenerateSingularityDef() to S3
// and adds a job to wr to build the image. You'll need a wr manager running
// that can run jobs with root and access the S3, ie. a cloud deployment.
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
	singDef, err := b.generateSingularityDef(def)
	if err != nil {
		return "", err
	}

	singDefUploadPath := filepath.Join(s3Path, singularityDefBasename)

	err = b.s3.UploadData(strings.NewReader(singDef), singDefUploadPath)

	return singDef, err
}

// generateSingularityDef uses our configured S3 binary cache and custom spack
// repo details to create a singularity definition file that will use Spack to
// build the Packages in the Definition.
func (b *Builder) generateSingularityDef(def *Definition) (string, error) {
	var w strings.Builder
	err := singularityTmpl.Execute(&w, &templateVars{
		S3BinaryCache:    b.config.S3.BinaryCache,
		RepoURL:          b.config.CustomSpackRepo.URL,
		RepoRef:          b.config.CustomSpackRepo.Ref,
		SpackBinaryCache: b.config.Spack.BinaryCache,
		BuildImage:       b.config.Spack.BuildImage,
		FinalImage:       b.config.Spack.FinalImage,
		ExtraExes:        def.Interpreters(),
		Packages:         def.Packages,
	})

	return w.String(), err
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
		b.addLogToRepo(s3Path, def.FullEnvironmentPath())

		return err
	}

	exes, err := b.getExes(s3Path)
	if err != nil {
		return err
	}

	moduleFileData := def.ToModule(b.config.Module.ScriptsInstallDir, b.config.Module.Dependencies, exes)

	if err = b.prepareAndInstallArtifacts(def, s3Path, moduleFileData, exes); err != nil {
		return err
	}

	return b.prepareArtifactsFromS3AndSendToCoreAndS3(def, s3Path, moduleFileData, singDef, exes)
}

func (b *Builder) addLogToRepo(s3Path, environmentPath string) {
	log, err := b.s3.OpenFile(filepath.Join(s3Path, builderOut))
	if err != nil {
		slog.Error("error getting build log file", "err", err)

		return
	}

	if err := b.addArtifactsToRepo(map[string]io.Reader{
		builderOut: log,
	}, environmentPath); err != nil {
		slog.Error("error sending build log file to core", "err", err)
	}
}

func (b *Builder) getExes(s3Path string) ([]string, error) {
	exeData, err := b.s3.OpenFile(filepath.Join(s3Path, exesBasename))
	if err != nil {
		return nil, err
	}

	buf, err := io.ReadAll(exeData)
	if err != nil {
		return nil, err
	}

	return strings.Split(strings.TrimSpace(string(buf)), "\n"), nil
}

func (b *Builder) prepareAndInstallArtifacts(def *Definition, s3Path,
	moduleFileData string, exes []string) error {
	imageData, err := b.s3.OpenFile(filepath.Join(s3Path, imageBasename))
	if err != nil {
		return err
	}

	defer imageData.Close()

	return installModule(b.config.Module.ScriptsInstallDir, b.config.Module.ModuleInstallDir, def,
		strings.NewReader(moduleFileData), imageData, exes, b.config.Module.WrapperScript)
}

func (b *Builder) prepareArtifactsFromS3AndSendToCoreAndS3(def *Definition, s3Path,
	moduleFileData, singDef string, exes []string) error {
	logData, lockData, err := b.getArtifactDataFromS3(s3Path)
	if err != nil {
		return err
	}

	concreteSpackYAMLFile, err := SpackLockToSoftPackYML(lockData, def.Description, exes)
	if err != nil {
		return err
	}

	if err = b.s3.UploadData(strings.NewReader(concreteSpackYAMLFile), filepath.Join(s3Path, softpackYaml)); err != nil {
		return err
	}

	return b.addArtifactsToRepo(
		map[string]io.Reader{
			spackLock:              bytes.NewReader(lockData),
			softpackYaml:           strings.NewReader(concreteSpackYAMLFile),
			singularityDefBasename: strings.NewReader(singDef),
			builderOut:             logData,
			moduleForCoreBasename:  strings.NewReader(moduleFileData),
			usageBasename:          strings.NewReader(def.ModuleUsage(b.config.Module.LoadPath)),
		},
		def.FullEnvironmentPath(),
	)
}

func (b *Builder) getArtifactDataFromS3(s3Path string) (io.Reader, []byte, error) {
	logData, err := b.s3.OpenFile(filepath.Join(s3Path, builderOut))
	if err != nil {
		return nil, nil, err
	}

	lockFile, err := b.s3.OpenFile(filepath.Join(s3Path, spackLock))
	if err != nil {
		return nil, nil, err
	}

	lockData, err := io.ReadAll(lockFile)
	if err != nil {
		return nil, nil, err
	}

	return logData, lockData, nil
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

type softpackTemplateVars struct {
	Description []string
	Packages    []ConcreteSpec
	Exes        []string
}

func SpackLockToSoftPackYML(data []byte, desc string, exes []string) (string, error) {
	var sl SpackLock

	if err := json.Unmarshal(data, &sl); err != nil {
		return "", err
	}

	concreteSpecs := make([]ConcreteSpec, len(sl.Roots))

	for i, root := range sl.Roots {
		concrete, ok := sl.ConcreteSpecs[root.Hash]
		if !ok {
			return "", ErrInvalidJSON
		}

		concreteSpecs[i] = concrete
	}

	var sb strings.Builder

	if err := softpackTmpl.Execute(&sb, softpackTemplateVars{
		Description: strings.Split(desc, "\n"),
		Packages:    concreteSpecs,
		Exes:        exes,
	}); err != nil {
		return "", err
	}

	return sb.String(), nil
}

func (b *Builder) addArtifactsToRepo(artifacts map[string]io.Reader, envPath string) error { //nolint:misspell
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
	slog.Debug("addArtifactsToRepo", "url", b.config.CoreURL+"?"+url.QueryEscape(envPath), "err", err)

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
