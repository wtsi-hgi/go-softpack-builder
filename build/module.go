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
	_ "embed"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed module.tmpl
var moduleTmplStr string
var moduleTmpl *template.Template //nolint:gochecknoglobals

//go:embed usage.tmpl
var usageTmplStr string
var usageTmpl *template.Template //nolint:gochecknoglobals

func init() { //nolint:gochecknoinits
	moduleTmpl = template.Must(template.New("").Parse(moduleTmplStr))
	usageTmpl = template.Must(template.New("").Parse(usageTmplStr))
}

// ToModule creates a tcl module based on our packages, and uses installDir to
// prepend a PATH for the exe wrapper scripts that will be at the installed
// location of the module. Any supplied module dependencies will be module
// loaded.
func (d *Definition) ToModule(installDir string, deps, exes []string) string {
	var sb strings.Builder

	moduleTmpl.Execute(&sb, struct { //nolint:errcheck
		InstallDir   string
		Dependencies []string
		*Definition
		Description []string
		Exes        []string
	}{
		InstallDir:   installDir,
		Dependencies: deps,
		Definition:   d,
		Description:  strings.Split(d.Description, "\n"),
		Exes:         exes,
	})

	return sb.String()
}

// ModuleUsage returns a markdown formatted usage that tells a user to module
// load our environment installed in the given loadPath.
func (d *Definition) ModuleUsage(loadPath string) string {
	var sb strings.Builder

	usageTmpl.Execute(&sb, filepath.Join( //nolint:errcheck
		loadPath, d.EnvironmentPath, d.EnvironmentName, d.EnvironmentVersion,
	))

	return sb.String()
}
