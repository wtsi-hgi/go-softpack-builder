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

package coremock

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
)

// MockCore can be used to bring up a simplified core-like service that you can
// upload and get files from.
type MockCore struct {
	mu    sync.RWMutex
	Err   error
	Files map[string]string
}

// NewMockCore returns a new MockCore with an empty set of Files.
func NewMockCore() *MockCore {
	return &MockCore{
		Files: make(map[string]string),
	}
}

func (m *MockCore) setFile(filename, contents string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Files[filename] = contents
}

// GetFile thread-safe returns previously uploaded file contents by filename.
func (m *MockCore) GetFile(filename string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contents, ok := m.Files[filename]

	return contents, ok
}

// ServeHTTP is to implement http.Handler so you can httptest.NewServer(m).
func (m *MockCore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.Err != nil {
		http.Error(w, m.Err.Error(), http.StatusInternalServerError)

		return
	}

	mr, err := r.MultipartReader()
	if err != nil {
		return
	}

	envPath, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		return
	}

	m.readFileFromQuery(mr, envPath)
}

func (m *MockCore) readFileFromQuery(mr *multipart.Reader, envPath string) {
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return
		}

		name := p.FileName()

		buf, err := io.ReadAll(p)
		if err != nil {
			return
		}

		m.setFile(filepath.Join(envPath, name), string(buf))
	}
}
