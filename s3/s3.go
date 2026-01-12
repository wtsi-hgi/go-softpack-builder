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
	"path"

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

// OpenFile lets you stream the given S3 bucket/source object.
func (s *S3) OpenFile(source string) (io.ReadCloser, error) {
	source = s.RemotePath(source)

	return s.S3Accessor.OpenFile(source, 0)
}

// RemoveFile removes the given path from S3 after checking it exists.
func (s *S3) RemoveFile(path string) error {
	path = s.RemotePath(path)

	if exists, err := s.FileExists(path); !exists {
		return os.ErrNotExist
	} else if err != nil {
		return err
	}

	if err := s.DeleteFile(path); err != nil {
		return err
	}

	return nil
}

// FileExists checks whether the given path exists in S3.
func (s *S3) FileExists(path_str string) (bool, error) {
	res, err := s.ListEntries(path.Dir(path_str) + "/")
	if err != nil {
		return false, err
	}

	for _, r := range res {
		if r.Name == path_str {
			return true, nil
		}
	}

	return false, nil
}
