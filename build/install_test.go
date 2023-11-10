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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestInstall(t *testing.T) {
	Convey("Given a Spack Definition, image file Reader, and a module file reader, install the files", t, func() {
		tmpScriptsDir := t.TempDir()
		tmpModulesDir := t.TempDir()

		def := getExampleDefinition()
		moduleFile := "some module file"
		imageFile := "some image file"

		// exes would normally be determined during the build process, and
		// wrapperScript would come from config
		exes := []string{"a", "b"}
		wrapperScript := "/path/to/wrapper.script"

		err := installModule(tmpScriptsDir, tmpModulesDir, def,
			strings.NewReader(moduleFile), strings.NewReader(imageFile), exes, wrapperScript)
		So(err, ShouldBeNil)

		createdModuleFile := readFile(t, filepath.Join(tmpModulesDir, def.EnvironmentPath,
			def.EnvironmentName, def.EnvironmentVersion))
		scriptsDir := filepath.Join(tmpScriptsDir, def.EnvironmentPath, def.EnvironmentName,
			def.EnvironmentVersion+ScriptsDirSuffix)
		createdImageFile := readFile(t, filepath.Join(scriptsDir, ImageBasename))

		So(createdModuleFile, ShouldEqual, moduleFile)
		So(createdImageFile, ShouldEqual, imageFile)

		for _, exe := range exes {
			dest, err := os.Readlink(filepath.Join(scriptsDir, exe))
			So(err, ShouldBeNil)
			So(dest, ShouldEqual, wrapperScript)
		}
	})
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	f, err := os.Open(path)
	So(err, ShouldBeNil)

	buf, err := io.ReadAll(f)
	So(err, ShouldBeNil)

	return string(buf)
}
