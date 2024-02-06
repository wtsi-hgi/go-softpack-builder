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

package reindex

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otiai10/copy"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/build"
	"github.com/wtsi-hgi/go-softpack-builder/config"
)

const userPerms = 0700

type fakeBuilder struct {
	cb func()
}

func (f *fakeBuilder) SetPostBuildCallback(cb func()) {
	f.cb = cb
}

func (f *fakeBuilder) buildCalled() time.Time {
	f.cb()

	return time.Now()
}

func TestReindex(t *testing.T) {
	_, err := exec.LookPath("spack")
	if err != nil {
		SkipConvey("skipping reindex tests since spack not in PATH", t, func() {})

		return
	}

	s3bucketPath := os.Getenv("GSB_S3_TEST_PATH")
	if s3bucketPath == "" {
		SkipConvey("skipping reindex tests since GSB_S3_TEST_PATH not set", t, func() {})

		return
	}

	Convey("build.Builder implements our Builder interface", t, func() {
		var _ Builder = (*build.Builder)(nil)
	})

	Convey("Given a conf, a builder and an unindexed spack binary cache", t, func() {
		tdir := t.TempDir()
		cacheDir := filepath.Join(tdir, "build_cache")
		err := os.Mkdir(cacheDir, userPerms)
		So(err, ShouldBeNil)

		sig1 := "linux-ubuntu22.04-x86_64_v3-gcc-11.4.0-berkeley-db-18.1.40-tr6lezmi6onfz2txkzowkh4qylmec2lk.spec.json.sig"
		err = copy.Copy(sig1, filepath.Join(cacheDir, sig1))
		So(err, ShouldBeNil)

		var conf config.Config
		conf.S3.BinaryCache = tdir
		conf.S3.BuildBase = "some_path"
		conf.Spack.ReindexHours = (10 * time.Millisecond).Hours()

		fb := &fakeBuilder{}

		Convey("We can schedule the reindex job", func() {
			s := NewScheduler(&conf, fb)
			s.Start()
			started := fb.buildCalled()
			defer s.Stop()

			index := getIndex(cacheDir, started)
			So(index, ShouldContainSubstring, "tr6lezmi6onfz2txkzowkh4qylmec2lk")

			sig2 := "linux-ubuntu22.04-x86_64_v3-gcc-11.4.0-curl-8.4.0-dnenyfmmx3fbiksufzhmb4qwjcvej7jg.spec.json.sig"
			err = copy.Copy(sig2, filepath.Join(cacheDir, sig2))
			So(err, ShouldBeNil)

			lastBuild := fb.buildCalled()
			<-time.After(hoursToDuration(conf.Spack.ReindexHours))
			index = getIndex(cacheDir, lastBuild)
			So(index, ShouldContainSubstring, "dnenyfmmx3fbiksufzhmb4qwjcvej7jg")
		})

		Convey("Index updates don't happen unless a build occurred recently", func() {
			s := NewScheduler(&conf, fb)
			s.Start()
			started := time.Now()
			defer s.Stop()

			index := getIndex(cacheDir, started)
			So(index, ShouldBeBlank)

			lastBuild := fb.buildCalled()
			<-time.After(hoursToDuration(conf.Spack.ReindexHours))
			index = getIndex(cacheDir, lastBuild)
			So(index, ShouldNotBeBlank)
		})

		Convey("Spack errors are logged", func() {
			logWriter := new(strings.Builder)
			slog.SetDefault(slog.New(slog.NewTextHandler(logWriter, nil)))

			conf.S3.BinaryCache = "/bad"
			s := NewScheduler(&conf, fb)
			s.Start()
			started := fb.buildCalled()
			defer s.Stop()

			getIndex(cacheDir, started)
			So(logWriter.String(), ShouldContainSubstring, `level=ERROR msg="spack reindex failed"`)
			So(logWriter.String(), ShouldContainSubstring, "file:///bad/build_cache: [Errno 2] No such file or directory")

			Convey("including when spack isn't available", func() {
				os.Setenv("PATH", "/garbage")
				started := fb.buildCalled()

				getIndex(cacheDir, started)
				So(logWriter.String(), ShouldContainSubstring, `level=ERROR msg="could not run spack buildcache update-index"`)
				So(logWriter.String(), ShouldContainSubstring, "executable file not found in $PATH")
			})
		})
	})
}

func getIndex(cacheDir string, minMtime time.Time) string {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(3 * time.Second)
	indexPath := filepath.Join(cacheDir, "index.json")

	for {
		select {
		case <-ticker.C:
			info, err := os.Stat(indexPath)
			if err != nil {
				continue
			}

			if info.ModTime().Before(minMtime) {
				continue
			}

			contents, err := os.ReadFile(indexPath)
			if err != nil {
				continue
			}

			return string(contents)
		case <-timeout:
			return ""
		}
	}
}
