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

package build

import (
	"io"
	"os"
	"path/filepath"
)

const (
	imageBasename    = "singularity.sif"
	scriptsDirSuffix = "-scripts"
	perms            = 0755
	flags            = os.O_EXCL | os.O_CREATE | os.O_WRONLY
)

func InstallModule(installBase string, def *Definition, module,
	image io.Reader, exes []string, wrapperScript string) (err error) {
	var installDir, scriptsDir string

	installDir, scriptsDir, err = makeModuleDirs(installBase, def)
	if err != nil {
		return err
	}

	modulePath := filepath.Join(installDir, def.EnvironmentVersion)

	defer func() {
		if err != nil {
			os.Remove(modulePath)
			os.RemoveAll(scriptsDir)
		}
	}()

	if err = installFile(module, modulePath); err != nil {
		return err
	}

	if err = installFile(image, filepath.Join(scriptsDir, imageBasename)); err != nil {
		return err
	}

	return createExeSymlinks(wrapperScript, scriptsDir, exes)
}

func makeModuleDirs(installBase string, def *Definition) (string, string, error) {
	installDir := filepath.Join(installBase, def.EnvironmentPath, def.EnvironmentName)

	if err := os.MkdirAll(installDir, perms); err != nil {
		return "", "", err
	}

	scriptsDir := filepath.Join(installDir, def.EnvironmentVersion+scriptsDirSuffix)

	if err := os.MkdirAll(scriptsDir, perms); err != nil {
		return "", "", err
	}

	return installDir, scriptsDir, nil
}

func installFile(data io.Reader, path string) error {
	f, err := os.OpenFile(path, flags, perms)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = io.Copy(f, data)

	return err
}

func createExeSymlinks(wrapperScript, scriptsDir string, exes []string) error {
	for _, exe := range exes {
		if err := os.Symlink(wrapperScript, filepath.Join(scriptsDir, exe)); err != nil {
			return err
		}
	}

	return nil
}
