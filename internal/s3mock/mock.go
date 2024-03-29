/*******************************************************************************
 * Copyright (c) 2024 Genome Research Ltd.
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

package s3mock

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/wtsi-hgi/go-softpack-builder/core"
	"github.com/wtsi-hgi/go-softpack-builder/internal"
)

const ErrS3Mock = internal.Error("Mock S3 error")

// MockS3 can be used to test a build.Builder by implementing the build.S3
// interface.
type MockS3 struct {
	Data        string
	Def         string
	SoftpackYML string
	Readme      string
	Fail        bool
	Exes        string
}

// UploadData implements the build.S3 interface.
func (m *MockS3) UploadData(data io.Reader, dest string) error {
	if m.Fail {
		return ErrS3Mock
	}

	buff, err := io.ReadAll(data)
	if err != nil {
		return err
	}

	switch filepath.Ext(dest) {
	case ".def":
		m.Data = string(buff)
		m.Def = dest
	case ".yml":
		m.SoftpackYML = string(buff)
	case ".md":
		m.Readme = string(buff)
	}

	return nil
}

// OpenFile implements the build.S3 interface.
func (m *MockS3) OpenFile(source string) (io.ReadCloser, error) {
	if filepath.Base(source) == core.ExesBasename {
		return io.NopCloser(strings.NewReader(m.Exes)), nil
	}

	if filepath.Base(source) == core.BuilderOut {
		return io.NopCloser(strings.NewReader("output")), nil
	}

	if filepath.Base(source) == core.SpackLockFile {
		return io.NopCloser(strings.NewReader(`{"_meta":{"file-type":"spack-lockfile","lockfile-version":5,"specfile-version":4},"spack":{"version":"0.21.0.dev0","type":"git","commit":"dac3b453879439fd733b03d0106cc6fe070f71f6"},"roots":[{"hash":"oibd5a4hphfkgshqiav4fdkvw4hsq4ek","spec":"xxhash arch=None-None-x86_64_v3"}, {"hash":"1ibd5a4hphfkgshqiav4fdkvw4hsq4e1","spec":"py-anndata arch=None-None-x86_64_v3"}, {"hash":"2ibd5a4hphfkgshqiav4fdkvw4hsq4e2","spec":"r-seurat arch=None-None-x86_64_v3"}],"concrete_specs":{"oibd5a4hphfkgshqiav4fdkvw4hsq4ek":{"name":"xxhash","version":"0.8.1","arch":{"platform":"linux","platform_os":"ubuntu22.04","target":"x86_64_v3"},"compiler":{"name":"gcc","version":"11.4.0"},"namespace":"builtin","parameters":{"build_system":"makefile","cflags":[],"cppflags":[],"cxxflags":[],"fflags":[],"ldflags":[],"ldlibs":[]},"package_hash":"wuj5b2kjnmrzhtjszqovcvgc3q46m6hoehmiccimi5fs7nmsw22a====","hash":"oibd5a4hphfkgshqiav4fdkvw4hsq4ek"},"2ibd5a4hphfkgshqiav4fdkvw4hsq4e2":{"name":"r-seurat","version":"4","arch":{"platform":"linux","platform_os":"ubuntu22.04","target":"x86_64_v3"},"compiler":{"name":"gcc","version":"11.4.0"},"namespace":"builtin","parameters":{"build_system":"makefile","cflags":[],"cppflags":[],"cxxflags":[],"fflags":[],"ldflags":[],"ldlibs":[]},"package_hash":"2uj5b2kjnmrzhtjszqovcvgc3q46m6hoehmiccimi5fs7nmsw222====","hash":"2ibd5a4hphfkgshqiav4fdkvw4hsq4e2"}, "1ibd5a4hphfkgshqiav4fdkvw4hsq4e1":{"name":"py-anndata","version":"3.14","arch":{"platform":"linux","platform_os":"ubuntu22.04","target":"x86_64_v3"},"compiler":{"name":"gcc","version":"11.4.0"},"namespace":"builtin","parameters":{"build_system":"makefile","cflags":[],"cppflags":[],"cxxflags":[],"fflags":[],"ldflags":[],"ldlibs":[]},"package_hash":"2uj5b2kjnmrzhtjszqovcvgc3q46m6hoehmiccimi5fs7nmsw222====","hash":"1ibd5a4hphfkgshqiav4fdkvw4hsq4e1"}}}`)), nil //nolint:lll
	}

	if filepath.Base(source) == core.ImageBasename {
		return io.NopCloser(strings.NewReader("image")), nil
	}

	return nil, io.ErrUnexpectedEOF
}
