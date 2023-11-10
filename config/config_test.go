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

package config

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/internal"
)

func TestConfig(t *testing.T) {
	Convey("Given a yaml config file, it can be Parse()d", t, func() {
		configData, err := internal.TestData.Open("testdata/config.yml")
		So(err, ShouldBeNil)

		config, err := Parse(configData)
		So(err, ShouldBeNil)

		So(config.S3.BinaryCache, ShouldEqual, "spack")
		So(config.S3.BuildBase, ShouldEqual, "spack/builds")
		So(config.Module.ModuleInstallDir, ShouldEqual, "/software/modules/HGI/softpack")
		So(config.Module.ScriptsInstallDir, ShouldEqual, "/software/hgi/softpack/installs")
		So(config.Module.LoadPath, ShouldEqual, "HGI/softpack")
		So(config.Module.WrapperScript, ShouldEqual, "/path/to/wrapper/script")
		So(config.Module.Dependencies, ShouldResemble, []string{"/software/modules/ISG/singularity/3.10.0"})
		So(config.CustomSpackRepo, ShouldEqual, "https://github.com/org/spack")
		So(config.Spack.BinaryCache, ShouldEqual, "https://binaries.spack.io/develop")
		So(config.Spack.BuildImage, ShouldEqual, "spack/ubuntu-jammy:latest")
		So(config.Spack.FinalImage, ShouldEqual, "ubuntu:22.04")
		So(config.Spack.ProcessorTarget, ShouldEqual, "x86_64_v4")
		So(config.CoreURL, ShouldEqual, "http://x.y.z:9837/softpack")
		So(config.ListenURL, ShouldEqual, "localhost:2456")
	})
}
