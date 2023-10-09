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
	"net/http/httptest"
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/internal/gitmock"
)

func TestGetLatestCommit(t *testing.T) {
	Convey("Given a mock git server", t, func() {
		mg, commitHash := gitmock.New()
		ts := httptest.NewServer(mg)

		Convey("you can retrieve latest commit hash on primary branch from a dumb server", func() {
			commit, err := GetLatestCommit(ts.URL)
			So(err, ShouldBeNil)
			So(commit, ShouldEqual, commitHash)
		})

		Convey("you can retrieve latest commit hash on primary branch from a smart server", func() {
			mg.Smart = true

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
