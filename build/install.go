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
	"strings"

	"github.com/wtsi-hgi/go-softpack-builder/core"
	"github.com/wtsi-hgi/go-softpack-builder/internal"
)

const (
	ScriptsDirSuffix = "-scripts"
	ErrMakeDirectory = internal.Error("base not parent of leaf")

	perms = 0644
	flags = os.O_EXCL | os.O_CREATE | os.O_WRONLY
)

func installModule(scriptInstallBase, moduleInstallBase string, def *Definition, module,
	image io.Reader, exes []string, wrapperScript string) (err error) {
	var scriptsDir, moduleDir string

	scriptsDir, moduleDir, err = makeModuleDirs(scriptInstallBase, moduleInstallBase, def)
	if err != nil {
		return err
	}

	modulePath := filepath.Join(moduleDir, def.EnvironmentVersion)

	defer func() {
		if err != nil {
			os.Remove(modulePath)
			os.RemoveAll(scriptsDir)
		}
	}()

	if err = installFile(module, modulePath); err != nil {
		return err
	}

	if err = installFile(image, filepath.Join(scriptsDir, core.ImageBasename)); err != nil {
		return err
	}

	return createExeSymlinks(wrapperScript, scriptsDir, exes)
}

func makeModuleDirs(scriptInstallBase, moduleInstallBase string, def *Definition) (string, string, error) {
	scriptsDir := ScriptsDirFromNameAndVersion(scriptInstallBase, def.EnvironmentPath,
		def.EnvironmentName, def.EnvironmentVersion)
	moduleDir := ModuleDirFromName(moduleInstallBase, def.EnvironmentPath, def.EnvironmentName)

	if err := makeDirectory(scriptsDir, scriptInstallBase); err != nil {
		return "", "", err
	}

	if err := makeDirectory(moduleDir, moduleInstallBase); err != nil {
		return "", "", err
	}

	return scriptsDir, moduleDir, nil
}

// ScriptsDirFromNameAndVersion returns the canonical scripts path for an
// environment.
func ScriptsDirFromNameAndVersion(scriptInstallBase, path, name, version string) string {
	return filepath.Join(scriptInstallBase, path,
		name, version+ScriptsDirSuffix)
}

// ModuleDirFromName returns the canonical module path for an environment.
func ModuleDirFromName(moduleInstallBase, path, name string) string {
	return filepath.Join(moduleInstallBase, path, name)
}

// makeDirectory does a MkdirAll for leafDir, and then makes sure it and it's
// parents up to baseDir are world accesible.
func makeDirectory(leafDir, baseDir string) error {
	leafDir, err := filepath.Abs(leafDir)
	if err != nil {
		return err
	}

	if baseDir, err = filepath.Abs(baseDir); err != nil {
		return err
	}

	if !strings.HasPrefix(leafDir, baseDir) {
		return ErrMakeDirectory
	}

	if err = os.MkdirAll(leafDir, perms); err != nil {
		return err
	}

	for leafDir != baseDir {
		if err := os.Chmod(leafDir, perms); err != nil {
			return err
		}

		leafDir = filepath.Dir(leafDir)
	}

	return nil
}

func installFile(data io.Reader, path string) (err error) {
	var f *os.File

	f, err = os.OpenFile(path, flags, perms)
	if err != nil {
		return err
	}

	defer func() {
		if errr := f.Close(); err == nil {
			err = errr
		}
	}()

	if _, err = io.Copy(f, data); err != nil {
		return err
	}

	err = os.Chmod(path, perms)

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
