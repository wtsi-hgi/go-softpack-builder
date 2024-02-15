/*******************************************************************************
 * Copyright (c) 2023, 2024 Genome Research Ltd.
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
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/git"
	"github.com/wtsi-hgi/go-softpack-builder/internal"
	"github.com/wtsi-hgi/go-softpack-builder/internal/core"
	"github.com/wtsi-hgi/go-softpack-builder/s3"
	"github.com/wtsi-hgi/go-softpack-builder/wr"
)

const (
	uploadEndpoint = "/upload"
	ErrBuildFailed = "environment build failed"
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

const (
	ErrInvalidJSON         = internal.Error("invalid spack lock JSON")
	ErrEnvironmentBuilding = internal.Error("build already running for environment")

	ErrInvalidEnvPath = internal.Error("invalid environment path")
	ErrInvalidVersion = internal.Error("environment version required")
	ErrNoPackages     = internal.Error("packages required")
	ErrNoPackageName  = internal.Error("package names required")
)

// Package describes the name and optional version of a spack package.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
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

type S3 interface {
	UploadData(data io.Reader, dest string) error
	OpenFile(source string) (io.ReadCloser, error)
}

type Runner interface {
	Add(deployment string) (string, error)
	WaitForRunning(id string) error
	Wait(id string) (wr.WRJobStatus, error)
	Status(id string) (wr.WRJobStatus, error)
}

// The status of an individual build – when it was requested, when it started
// actually being built, and when its build finished.
type Status struct {
	Name       string
	Requested  *time.Time
	BuildStart *time.Time
	BuildDone  *time.Time
}

// Builder lets you do builds given config, S3 and a wr runner.
type Builder struct {
	config *config.Config
	s3     S3
	runner Runner

	mu                  sync.Mutex
	runningEnvironments map[string]bool

	postBuildMu sync.RWMutex
	postBuild   func()

	statusMu sync.RWMutex
	statuses map[string]*Status

	runnerPollInterval time.Duration
}

// New takes the s3 build cache URL, the repo and checkout reference of your
// custom spack repo, and returns a Builder. Optionally, supply objects that
// satisfy the S3 and Runner interfaces; if nil, these default to using the s3
// and wr packages.
func New(config *config.Config, s3helper S3, runner Runner) (*Builder, error) {
	if s3helper == nil {
		var err error

		s3helper, err = s3.New(config.S3.BuildBase)
		if err != nil {
			return nil, err
		}
	}

	if runner == nil {
		runner = wr.New(config.WRDeployment)
	}

	return &Builder{
		config:              config,
		s3:                  s3helper,
		runner:              runner,
		runningEnvironments: make(map[string]bool),
		statuses:            make(map[string]*Status),
		runnerPollInterval:  1 * time.Second,
	}, nil
}

type templateVars struct {
	S3BinaryCache   string
	RepoURL         string
	RepoRef         string
	ProcessorTarget string
	BuildImage      string
	FinalImage      string
	ExtraExes       []string
	Packages        []Package
}

// SetPostBuildCallback causes the passed callback to be called after the
// spack-related parts of a build have completed.
func (b *Builder) SetPostBuildCallback(cb func()) {
	b.postBuildMu.Lock()
	defer b.postBuildMu.Unlock()
	b.postBuild = cb
}

// Status returns the status of all known builds.
func (b *Builder) Status() []Status {
	b.statusMu.RLock()
	defer b.statusMu.RUnlock()

	statuses := make([]Status, 0, len(b.statuses))

	for _, status := range b.statuses {
		statuses = append(statuses, *status)
	}

	return statuses
}

// Build uploads a singularity.def generated by GenerateSingularityDef() to S3
// and adds a job to wr to build the image. You'll need a wr manager running
// that can run jobs with root and access the S3, ie. a cloud deployment.
func (b *Builder) Build(def *Definition) (err error) {
	b.buildStatus(def)

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

	hash := fmt.Sprintf("%X", sha256.Sum256([]byte(singDef)))

	singDefParentPath := filepath.Join(b.config.S3.BuildBase, s3Path)

	wrInput, err = wr.SingularityBuildInS3WRInput(singDefParentPath, hash)
	if err != nil {
		return err
	}

	go b.startBuild(def, wrInput, s3Path, singDef, singDefParentPath)

	return nil
}

func (b *Builder) buildStatus(def *Definition) *Status {
	b.statusMu.Lock()
	defer b.statusMu.Unlock()

	name := filepath.Join(def.EnvironmentPath, def.EnvironmentName) + "-" + def.EnvironmentVersion

	status, exists := b.statuses[name]
	if !exists {
		now := time.Now()
		status = &Status{
			Name:      name,
			Requested: &now,
		}

		b.statuses[name] = status
	}

	return status
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

	singDefUploadPath := filepath.Join(s3Path, core.SingularityDefBasename)

	err = b.s3.UploadData(strings.NewReader(singDef), singDefUploadPath)

	return singDef, err
}

// generateSingularityDef uses our configured S3 binary cache and custom spack
// repo details to create a singularity definition file that will use Spack to
// build the Packages in the Definition.
func (b *Builder) generateSingularityDef(def *Definition) (string, error) {
	repoRef, err := git.GetLatestCommit(b.config.CustomSpackRepo)
	if err != nil {
		return "", err
	}

	var w strings.Builder
	err = singularityTmpl.Execute(&w, &templateVars{
		S3BinaryCache:   b.config.S3.BinaryCache,
		RepoURL:         b.config.CustomSpackRepo,
		RepoRef:         repoRef,
		ProcessorTarget: b.config.Spack.ProcessorTarget,
		BuildImage:      b.config.Spack.BuildImage,
		FinalImage:      b.config.Spack.FinalImage,
		ExtraExes:       def.Interpreters(),
		Packages:        def.Packages,
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
	status := b.buildStatus(def)

	jobID, err := b.runner.Add(wrInput)
	if err != nil {
		return err
	}

	err = b.runner.WaitForRunning(jobID)
	if err != nil {
		return err
	}

	b.statusMu.Lock()
	buildStart := time.Now()
	status.BuildStart = &buildStart
	b.statusMu.Unlock()

	wrStatus, err := b.runner.Wait(jobID)

	b.statusMu.Lock()
	buildDone := time.Now()
	status.BuildDone = &buildDone
	b.statusMu.Unlock()

	b.postBuildMu.RLock()
	if b.postBuild != nil {
		// if spack ran at all, it might've pushed things to the cache, even if
		// it didn't succeed or if later steps don't run
		b.postBuild()
	}
	b.postBuildMu.RUnlock()

	if err != nil || wrStatus != wr.WRJobStatusComplete {
		b.addLogToRepo(s3Path, def.FullEnvironmentPath())

		if err == nil {
			err = internal.Error(ErrBuildFailed)
		}

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
	log, err := b.s3.OpenFile(filepath.Join(s3Path, core.BuilderOut))
	if err != nil {
		slog.Error("error getting build log file", "err", err)

		return
	}

	if err := b.addArtifactsToRepo(map[string]io.Reader{
		core.BuilderOut: log,
	}, environmentPath); err != nil {
		slog.Error("error sending build log file to core", "err", err)
	}
}

func (b *Builder) getExes(s3Path string) ([]string, error) {
	exeData, err := b.s3.OpenFile(filepath.Join(s3Path, core.ExesBasename))
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
	imageData, err := b.s3.OpenFile(filepath.Join(s3Path, core.ImageBasename))
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

	concreteSpackYAMLFile, err := b.generateAndUploadSoftpackYAML(lockData, def.Description, exes, s3Path)
	if err != nil {
		return err
	}

	readme, err := b.generateAndUploadUsageFile(def, s3Path)
	if err != nil {
		return err
	}

	return b.addArtifactsToRepo(
		map[string]io.Reader{
			core.SpackLockFile:          bytes.NewReader(lockData),
			core.SoftpackYaml:           strings.NewReader(concreteSpackYAMLFile),
			core.SingularityDefBasename: strings.NewReader(singDef),
			core.BuilderOut:             logData,
			core.ModuleForCoreBasename:  strings.NewReader(moduleFileData),
			core.UsageBasename:          strings.NewReader(readme),
		},
		def.FullEnvironmentPath(),
	)
}

func (b *Builder) getArtifactDataFromS3(s3Path string) (io.Reader, []byte, error) {
	logData, err := b.s3.OpenFile(filepath.Join(s3Path, core.BuilderOut))
	if err != nil {
		return nil, nil, err
	}

	lockFile, err := b.s3.OpenFile(filepath.Join(s3Path, core.SpackLockFile))
	if err != nil {
		return nil, nil, err
	}

	lockData, err := io.ReadAll(lockFile)
	if err != nil {
		return nil, nil, err
	}

	return logData, lockData, nil
}

func (b *Builder) generateAndUploadSoftpackYAML(lockData []byte, description string,
	exes []string, s3Path string) (string, error) {
	concreteSoftpackYAMLFile, err := SpackLockToSoftPackYML(lockData, description, exes)
	if err != nil {
		return "", err
	}

	if err = b.s3.UploadData(strings.NewReader(concreteSoftpackYAMLFile),
		filepath.Join(s3Path, core.SoftpackYaml)); err != nil {
		return "", err
	}

	return concreteSoftpackYAMLFile, nil
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

// SpackLockToSoftPackYML uses the given spackLockData to generate a
// disambiguated softpack.yml file.
//
// The format of the file is as follows:
// description: |
//
//	$desc
//	The following executables are added to your PATH:
//	  - supplied_executable_1
//	  - supplied_executable_2
//	  - ...
//
// packages:
//   - supplied_package_1@v1
//   - supplied_package_2@v1.1
//   - ...
func SpackLockToSoftPackYML(spackLockData []byte, desc string, exes []string) (string, error) {
	var sl SpackLock

	if err := json.Unmarshal(spackLockData, &sl); err != nil {
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

func (b *Builder) generateAndUploadUsageFile(def *Definition, s3Path string) (string, error) {
	readme := def.ModuleUsage(b.config.Module.LoadPath)

	if err := b.s3.UploadData(strings.NewReader(readme), filepath.Join(s3Path, core.UsageBasename)); err != nil {
		return "", err
	}

	return readme, nil
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

	uploadURL := strings.TrimSuffix(b.config.CoreURL, "/") + uploadEndpoint + "?" + url.QueryEscape(envPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, pr)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	slog.Debug("addArtifactsToRepo", "url", b.config.CoreURL+uploadEndpoint+"?"+url.QueryEscape(envPath), "err", err)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		var sb strings.Builder

		io.Copy(&sb, resp.Body) //nolint:errcheck

		return internal.Error(sb.String())
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
