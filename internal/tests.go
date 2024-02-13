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

package internal

import (
	"embed"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wtsi-hgi/go-softpack-builder/internal/core"
	"github.com/wtsi-hgi/go-softpack-builder/wr"
)

//go:embed testdata
var TestData embed.FS

// ConcurrentStringBuilder should be used when testing slog logging:
//
//	var logWriter internal.ConcurrentStringBuilder
//	slog.SetDefault(slog.New(slog.NewTextHandler(&logWriter, nil)))
type ConcurrentStringBuilder struct {
	mu sync.RWMutex
	strings.Builder
}

// Write is for implementing io.Writer.
func (c *ConcurrentStringBuilder) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Builder.Write(p)
}

// WriteString is for implementing io.Writer.
func (c *ConcurrentStringBuilder) WriteString(str string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Builder.WriteString(str)
}

// String returns the current logs messages as a string.
func (c *ConcurrentStringBuilder) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Builder.String()
}

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

const ErrMock = Error("Mock error")

// UploadData implements the build.S3 interface.
func (m *MockS3) UploadData(data io.Reader, dest string) error {
	if m.Fail {
		return ErrMock
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

// MockWR can be used to test a build.Builder without having real wr running.
type MockWR struct {
	Ch                    chan struct{}
	Cmd                   string
	Fail                  bool
	PollForStatusInterval time.Duration
	JobDuration           time.Duration

	sync.RWMutex
	ReturnStatus wr.WRJobStatus
}

func NewMockWR(pollForStatusInterval, jobDuration time.Duration) *MockWR {
	return &MockWR{
		Ch:                    make(chan struct{}),
		PollForStatusInterval: pollForStatusInterval,
		JobDuration:           jobDuration,
	}
}

// Add implements build.Runner interface.
func (m *MockWR) Add(cmd string) (string, error) { //nolint:unparam
	m.Cmd = cmd

	return "abc123", nil
}

// SetRunning should be used before waiting on Ch and if you need to test
// BuildStart time in Status.
func (m *MockWR) SetRunning() {
	m.Lock()
	defer m.Unlock()

	m.ReturnStatus = wr.WRJobStatusRunning
}

// WaitForRunning implements build.Runner interface.
func (m *MockWR) WaitForRunning(string) error { //nolint:unparam
	for {
		m.RLock()
		rs := m.ReturnStatus
		m.RUnlock()

		if rs == wr.WRJobStatusRunning || rs == wr.WRJobStatusBuried || rs == wr.WRJobStatusComplete {
			return nil
		}

		<-time.After(m.PollForStatusInterval)
	}
}

// Wait implements build.Runner interface.
func (m *MockWR) Wait(string) (wr.WRJobStatus, error) {
	defer close(m.Ch)
	<-time.After(m.JobDuration)

	if m.Fail {
		return wr.WRJobStatusBuried, ErrMock
	}

	return wr.WRJobStatusComplete, nil
}

// Status implements build.Runner interface.
func (m *MockWR) Status(string) (wr.WRJobStatus, error) { //nolint:unparam
	m.RLock()
	defer m.RUnlock()

	return m.ReturnStatus, nil
}
