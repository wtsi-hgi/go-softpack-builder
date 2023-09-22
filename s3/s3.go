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

	"github.com/VertebrateResequencing/muxfys"
)

// S3 lets you upload data to S3 and retrieve it.
type S3 struct {
	*muxfys.S3Accessor
}

// New returns an S3 that gets your S3 credentials from ~/.s3cfg. The bucketPath
// will be checked for accessibility. Only the first "directory" of the path,
// actual bucket name, will be checked and stored as a root for the other method
// paths.
func New(bucketPath string) (*S3, error) {
	config, err := muxfys.S3ConfigFromEnvironment("", bucketPath)
	if err != nil {
		return nil, err
	}

	accessor, err := muxfys.NewS3Accessor(config)
	if err != nil {
		return nil, err
	}

	return &S3{S3Accessor: accessor}, nil
}

// UploadData uploads the given data to bucket/dest.
func (s *S3) UploadData(data io.Reader, dest string) error {
	dest = s.RemotePath(dest)

	return s.S3Accessor.UploadData(data, dest)
}

// DownloadFile downloads the given S3 bucket/source object to dest path on
// local disk.
func (s *S3) DownloadFile(source, dest string) error {
	source = s.RemotePath(source)

	return s.S3Accessor.DownloadFile(source, dest)
}

// OpenFile lets you stream the given S3 bucket/source object.
func (s *S3) OpenFile(source string) (io.ReadCloser, error) {
	source = s.RemotePath(source)

	return s.S3Accessor.OpenFile(source, 0)
}
