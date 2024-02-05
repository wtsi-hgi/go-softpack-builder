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
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/otiai10/copy"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/config"
)

const userPerms = 0700

type fakeBuilder struct {
	t time.Time
}

func (f *fakeBuilder) LastBuiltTime() time.Time {
	return f.t
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
			fb.t = time.Now().Add(-1 * time.Second)
			s := NewScheduler(&conf, fb)
			s.Start()
			started := time.Now()
			defer s.Stop()

			index := getIndex(cacheDir, started)
			So(index, ShouldContainSubstring, "tr6lezmi6onfz2txkzowkh4qylmec2lk")

			sig2 := "linux-ubuntu22.04-x86_64_v3-gcc-11.4.0-curl-8.4.0-dnenyfmmx3fbiksufzhmb4qwjcvej7jg.spec.json.sig"
			err = copy.Copy(sig2, filepath.Join(cacheDir, sig2))
			So(err, ShouldBeNil)

			fb.t = time.Now()
			<-time.After(hoursToDuration(conf.Spack.ReindexHours))
			index = getIndex(cacheDir, fb.t)
			So(index, ShouldContainSubstring, "dnenyfmmx3fbiksufzhmb4qwjcvej7jg")
		})

		Convey("Index updates don't happen unless a build occurred recently", func() {
			s := NewScheduler(&conf, fb)
			s.Start()
			started := time.Now()
			defer s.Stop()

			index := getIndex(cacheDir, started)
			So(index, ShouldBeBlank)

			fb.t = time.Now()
			<-time.After(hoursToDuration(conf.Spack.ReindexHours))
			index = getIndex(cacheDir, fb.t)
			So(index, ShouldNotBeBlank)
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
