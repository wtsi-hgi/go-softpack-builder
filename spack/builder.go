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
	buildCache         string
	customSpackRepoURL string
	customSpackRepoRef string
}

// New takes the s3 build cache URL, the repo and checkout reference of your
// custom spack repo, and returns a Builder.
func New(buildCache, repoURL, repoRef string) *Builder {
	return &Builder{
		buildCache:         buildCache,
		customSpackRepoURL: repoURL,
		customSpackRepoRef: repoRef,
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
		BuildCache: b.buildCache,
		RepoURL:    b.customSpackRepoURL,
		RepoRef:    b.customSpackRepoRef,
		Packages:   def.Packages,
	})

	return w.String(), err
}
