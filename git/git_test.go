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

package git

import (
	crypto "crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

const (
	ErrNotFound = Error("not found")
)

type mockGit struct {
	refs       map[string]string
	masterName string
	smart      bool
}

func newMockGit() (*mockGit, string) {
	numRefs := rand.Intn(5) + 5 //nolint:gosec

	refs := make(map[string]string, numRefs)

	var (
		masterName, masterCommit string
		hash                     [20]byte
	)

	for i := 0; i < numRefs; i++ {
		randChars := make([]byte, rand.Intn(5)+5) //nolint:gosec
		crypto.Read(randChars)                    //nolint:errcheck
		crypto.Read(hash[:])                      //nolint:errcheck

		masterName = base64.RawStdEncoding.EncodeToString(randChars)
		masterCommit = fmt.Sprintf("%020X", hash)

		refs[masterName] = masterCommit
	}

	return &mockGit{
		refs:       refs,
		masterName: masterName,
	}, masterCommit
}

func (m *mockGit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := m.handle(w, r.URL.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (m *mockGit) handle(w http.ResponseWriter, path string) error {
	switch path {
	case refsPath:
		if m.smart {
			w.Header().Set("Content-Type", smartContentType)

			return m.handleSmartRefs(w)
		}

		return m.handleRefs(w)
	case headPath:
		return m.handleHead(w)
	}

	return ErrNotFound
}

func (m *mockGit) handleSmartRefs(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "%s002D%s %s\n", expectedHeader, m.refs[m.masterName], headRef); err != nil {
		return err
	}

	for ref, commit := range m.refs {
		if _, err := fmt.Fprintf(w, "%04X%s %s\n", 41+len(ref), commit, ref); err != nil {
			return err
		}
	}

	_, err := io.WriteString(w, "0000")

	return err
}

func (m *mockGit) handleRefs(w io.Writer) error {
	for ref, commit := range m.refs {
		if _, err := fmt.Fprintf(w, "%s\t%s\n", commit, ref); err != nil {
			return err
		}
	}

	return nil
}

func (m *mockGit) handleHead(w io.Writer) error {
	_, err := io.WriteString(w, "ref: "+m.masterName)

	return err
}

func TestGetLatestCommit(t *testing.T) {
	Convey("Given a mock git server", t, func() {
		mg, commitHash := newMockGit()
		ts := httptest.NewServer(mg)

		Convey("you can retrieve latest commit hash on primary branch from a dumb server", func() {
			commit, err := GetLatestCommit(ts.URL)
			So(err, ShouldBeNil)
			So(commit, ShouldEqual, commitHash)
		})

		Convey("you can retrieve latest commit hash on primary branch from a smart server", func() {
			mg.smart = true

			commit, err := GetLatestCommit(ts.URL)
			So(err, ShouldBeNil)
			So(commit, ShouldEqual, commitHash)
		})
	})

	repoURL := os.Getenv("GSB_TEST_REPO_URL")
	repoCommit := os.Getenv("GSB_TEST_REPO_COMMIT")

	if repoURL == "" || repoCommit == "" {
		SkipConvey("real test skipped, set GSB_TEST_REPO_URL and GSB_TEST_REPO_COMMIT to enable.", t, func() {})

		return
	}

	Convey("Given a URL to a real git server you can retrieve the latest hash from the primary branch", t, func() {
		commit, err := GetLatestCommit(repoURL)
		So(err, ShouldBeNil)
		So(commit, ShouldEqual, repoCommit)
	})
}
