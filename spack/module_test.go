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
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestModule(t *testing.T) {
	Convey("Given a Definition, you can generate a module file", t, func() {
		// moduleDependencies and installDir would come from our config yml
		moduleDependencies := "/software/modules/ISG/singularity/3.10.0"
		installDir := "/software/modules/HGI/softpack"

		def := getExampleDefinition()
		moduleFileData := def.ToModule(installDir, []string{moduleDependencies})
		So(moduleFileData, ShouldEqual, fmt.Sprintf(`#%%Module

proc ModulesHelp { } {
	puts stderr "%s"
}

module-whatis "Name: %s"
module-whatis "Version: %s"
module-whatis "Packages: xxhash@0.8.1, r-seurat@4"

module load %s

prepend-path PATH "%s/%s/%s/%s-scripts"
`, def.Description, def.EnvironmentName, def.EnvironmentVersion,
			moduleDependencies, installDir, def.EnvironmentPath,
			def.EnvironmentName, def.EnvironmentVersion))
	})

	Convey("Given a Definition, you can generate a Usage for a module file", t, func() {
		// moduleLoadPath would come from our config yml
		moduleLoadPath := "HGI/softpack"

		def := getExampleDefinition()
		usageFileData := def.ModuleUsage(moduleLoadPath)
		So(usageFileData, ShouldEqual, `# Usage

To use this environment, run:

`+"```"+`
module load HGI/softpack/groups/hgi/xxhash/0.8.1
`+"```"+`

This will usually add your desired software to your PATH. Check the description
of the environment for more information, which might also be available by
running:

`+"```"+`
module help HGI/softpack/groups/hgi/xxhash/0.8.1
`+"```\n")
	})
}
