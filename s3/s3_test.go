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

package s3

import (
	"io"
	"os"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestS3(t *testing.T) {
	s3bucketPath := os.Getenv("GSB_S3_TEST_PATH")
	if s3bucketPath == "" {
		SkipConvey("skipping S3 tests since GSB_S3_TEST_PATH not set", t, func() {})

		return
	}

	basePath := strings.Join(strings.Split(s3bucketPath[1:], "/")[1:], "/")

	Convey("Given a New S3 object that uses config on disk", t, func() {
		s3, err := New(s3bucketPath)
		So(err, ShouldBeNil)
		So(s3, ShouldNotBeNil)

		Convey("You can upload a string as a file", func() {
			basename := "test.txt"
			testData := "test"

			err = s3.UploadData(strings.NewReader(testData), basename)
			So(err, ShouldBeNil)

			defer s3.DeleteFile(basePath + "/" + basename) //nolint:errcheck

			entries, err := s3.ListEntries(basePath + "/")
			So(err, ShouldBeNil)
			So(len(entries), ShouldEqual, 1)
			So(entries[0].Name, ShouldEqual, basePath+"/"+basename)

			Convey("And then open it", func() {
				f, err := s3.OpenFile(basename)
				So(err, ShouldBeNil)

				defer f.Close()

				buf, err := io.ReadAll(f)
				So(err, ShouldBeNil)
				So(string(buf), ShouldEqual, testData)
			})

			Convey("And remove it", func() {
				err := s3.RemoveFile(basename)
				So(err, ShouldBeNil)
			})
		})
	})
}
