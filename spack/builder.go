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
	"strings"
	"text/template"

	"github.com/wtsi-hgi/go-softpack-builder/config"
)

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
}

// New takes the s3 build cache URL, the repo and checkout reference of your
// custom spack repo, and returns a Builder.
func New(config *config.Config) *Builder {
	return &Builder{
		config: config,
	}
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
	// singDef, err := b.GenerateSingularityDef(def)
	// if err != nil {
	// 	return err
	// }

	return nil
}
