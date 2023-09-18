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

package spack

import (
	_ "embed"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/s3"
	"github.com/wtsi-hgi/go-softpack-builder/wr"
)

const singularityDefBasename = "singularity.def"

//go:embed singularity.tmpl
var singularityTmplStr string
var singularityTmpl *template.Template

func init() {
	singularityTmpl = template.Must(template.New("").Parse(singularityTmplStr))
}

type Package struct {
	Name    string
	Version string

	// Exe is the command line a user would run to use this software
	Exe string
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
		b.config.S3.BuildBase, def.EnvironmentPath, def.EnvironmentName,
		def.EnvironmentVersion,
	)

	err = b.s3.UploadData(strings.NewReader(singDef), filepath.Join(
		s3Path, singularityDefBasename,
	))
	if err != nil {
		return err
	}

	wrInput, err := wr.SingularityBuildInS3WRInput(s3Path)
	if err != nil {
		return err
	}

	go func() {
		errb := b.asyncBuild(def, wrInput)
		if errb != nil {
			slog.Error("Async part of build failed", "err", errb.Error(), "s3Path", s3Path)
		}
	}()

	return nil
}

func (b *Builder) asyncBuild(def *Definition, wrInput string) error {
	err := b.runner.Run(wrInput)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir)

	imagePath := filepath.Join(tmpDir, "sif")

	err = b.s3.DownloadFile("singularity.sif", imagePath)
	if err != nil {
		return err
	}

	// and for spack.lock file

	moduleFileData := def.ToModule(b.config.Module.Dependencies)
	// usageFileData := def.ModuleUsage(b.config.Module.LoadPath)

	imageFile, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer imageFile.Close()

	err = InstallModule(b.config.Module.InstallDir, def,
		strings.NewReader(moduleFileData), imageFile)

	// SpackLockToSoftPackYML and AddArtifactsToRepo

	return err
}
