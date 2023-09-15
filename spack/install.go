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

package spack

import (
	"io"
	"os"
	"path/filepath"
)

const (
	imageExt = ".sif"
	perms    = 0755
	flags    = os.O_EXCL | os.O_CREATE | os.O_WRONLY
)

func InstallModule(installBase string, def *Definition, module, image io.Reader) error {
	installDir := filepath.Join(installBase, def.EnvironmentPath, def.EnvironmentName)

	err := os.MkdirAll(installDir, perms)
	if err != nil {
		return err
	}

	modulePath := filepath.Join(installDir, def.EnvironmentVersion)
	f, err := os.OpenFile(modulePath, flags, perms)
	if err != nil {
		return err
	}

	err = copyOrCleanup(f, module)
	if err != nil {
		return err
	}

	imagePath := filepath.Join(installDir, def.EnvironmentVersion+imageExt)
	f, err = os.OpenFile(imagePath, flags, perms)
	if err != nil {
		os.Remove(modulePath)
		return err
	}

	return copyOrCleanup(f, image)
}

func copyOrCleanup(f *os.File, src io.Reader) error {
	_, err := io.Copy(f, src)
	if err != nil {
		f.Close()
		os.Remove(f.Name())

		return err
	}

	err = f.Close()
	if err != nil {
		os.Remove(f.Name())
	}

	return err
}
