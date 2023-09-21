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
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/config"
)

type Error string

func (e Error) Error() string { return string(e) }

const mockError = Error("Mock error")

type mockS3 struct {
	ch             chan struct{}
	data           string
	dest           string
	downloadSource string
	fail           bool
	exes           string
}

func (m *mockS3) UploadData(data io.Reader, dest string) error {
	defer close(m.ch)

	if m.fail {
		return mockError
	}

	buff, err := io.ReadAll(data)
	if err != nil {
		return err
	}

	m.data = string(buff)
	m.dest = dest

	return nil
}

func (m *mockS3) DownloadFile(source, dest string) error {
	m.downloadSource = source

	f, err := os.Create(dest)
	if err != nil {
		return err
	}

	_, err = f.WriteString("mock")
	if err != nil {
		return err
	}

	return f.Close()
}

func (m *mockS3) OpenFile(source string) (io.ReadCloser, error) {
	if filepath.Base(source) == exesBasename {
		return io.NopCloser(strings.NewReader(m.exes)), nil
	}

	return nil, io.EOF
}

type mockWR struct {
	ch   chan struct{}
	cmd  string
	fail bool
}

func (m *mockWR) Run(cmd string) error {
	defer close(m.ch)

	if m.fail {
		return mockError
	}

	m.cmd = cmd

	return nil
}

func TestBuilder(t *testing.T) {
	Convey("Given binary cache and spack repo details and a Definition", t, func() {
		var conf config.Config
		conf.S3.BinaryCache = "s3://spack"
		conf.S3.BuildBase = "some_path"
		conf.CustomSpackRepo.URL = "https://github.com/spack/repo"
		conf.CustomSpackRepo.Ref = "some_tag"

		ms3 := &mockS3{ch: make(chan struct{})}
		mwr := &mockWR{ch: make(chan struct{})}

		builder := &Builder{
			config: &conf,
			s3:     ms3,
			runner: mwr,
		}

		def := getExampleDefinition()

		Convey("You can generate a singularity .def", func() {
			def, err := builder.GenerateSingularityDef(def)

			So(err, ShouldBeNil)
			So(def, ShouldEqual, `Bootstrap: docker
From: spack/ubuntu-jammy:latest
Stage: build

%files
	/home/ubuntu/.aws /root/.aws
	/home/ubuntu/spack/opt/spack/gpg /opt/spack/opt/spack/gpg

%post
	# Create the manifest file for the installation in /opt/spack-environment
	mkdir /opt/spack-environment && cd /opt/spack-environment
	cat << EOF > spack.yaml
spack:
  # add package specs to the specs list
  specs:
  - xxhash@0.8.1 arch=None-None-x86_64_v3
  - r-seurat@4 arch=None-None-x86_64_v3
  view: /opt/view
  concretizer:
    unify: true
  config:
    install_tree: /opt/software
EOF

	# Install all the required software
	. /opt/spack/share/spack/setup-env.sh
	spack mirror add develop https://binaries.spack.io/develop
	spack mirror add s3cache "s3://spack"
	tmpDir="$(mktemp -d)"
	git clone "https://github.com/spack/repo" "$tmpDir"
	git -C "$tmpDir" checkout "some_tag"
	spack repo add "$tmpDir"
	spack buildcache keys --install --trust
	spack config add "config:install_tree:padded_length:128"
	spack -e . concretize
	spack -e . install
	spack -e . buildcache push --rebuild-index -f s3cache
	spack gc -y
	spack env activate --sh -d . >> /opt/spack-environment/environment_modifications.sh

	# Strip the binaries to reduce the size of the image
	find -L /opt/view/* -type f -exec readlink -f '{}' \; | \
	xargs file -i | \
	grep 'charset=binary' | \
	grep 'x-executable\|x-archive\|x-sharedlib' | \
	awk -F: '{print $1}' | xargs strip

	spack env activate .
	exes="$(find $(echo $PATH | tr ":" "\n" | grep /opt/view/ | tr "\n" " ") -maxdepth 1 -executable -type l | xargs -L 1 readlink)";
	for pkg in "xxhash" "r-seurat"; do
		echo "$exes" | grep "/linux-[^/]*/gcc-[^/]*/$pkg-";
	done | xargs -L 1 basename | sort | uniq > executables

Bootstrap: docker
From: ubuntu:22.04
Stage: final

%files from build
	/opt/spack-environment /opt
	/opt/software /opt
	/opt/._view /opt
	/opt/view /opt
	/opt/spack-environment/environment_modifications.sh /opt/spack-environment/environment_modifications.sh

%post
	# Modify the environment without relying on sourcing shell specific files at startup
	cat /opt/spack-environment/environment_modifications.sh >> $SINGULARITY_ENVIRONMENT
`)
		})

		var logWriter strings.Builder
		slog.SetDefault(slog.New(slog.NewTextHandler(&logWriter, nil)))

		Convey("You can do a Build", func() {
			conf.Module.InstallDir = t.TempDir()
			conf.Module.WrapperScript = "/path/to/wrapper"
			ms3.exes = "xxhsum\nxxh32sum\nxxh64sum\nxxh128sum\n"
			err := builder.Build(def)
			So(err, ShouldBeNil)

			<-ms3.ch
			So(ms3.dest, ShouldEqual, "groups/hgi/xxhash/0.8.1/singularity.def")
			So(ms3.data, ShouldContainSubstring, "specs:\n  - xxhash@0.8.1 arch=None-None-x86_64_v3\n  - r-seurat@4 arch=None-None-x86_64_v3\n  view")

			<-mwr.ch
			So(mwr.cmd, ShouldContainSubstring, "echo doing build in some_path/groups/hgi/xxhash/0.8.1; sudo singularity build")

			envPath := filepath.Join(conf.Module.InstallDir,
				def.EnvironmentPath, def.EnvironmentName)

			modulePath := filepath.Join(envPath, def.EnvironmentVersion)
			scriptsPath := filepath.Join(envPath, def.EnvironmentVersion+scriptsDirSuffix)
			imagePath := filepath.Join(scriptsPath, imageBasename)
			expectedExes := []string{"xxhsum", "xxh32sum", "xxh64sum", "xxh128sum"}

			expectedFiles := []string{modulePath, scriptsPath, imagePath}

			for _, exe := range expectedExes {
				expectedFiles = append(expectedFiles, filepath.Join(scriptsPath, exe))
			}

			ok := waitFor(func() bool {
				for _, path := range expectedFiles {
					if _, err = os.Lstat(path); err != nil {
						return false
					}
				}

				return true
			})
			So(logWriter.String(), ShouldBeBlank)
			So(ok, ShouldBeTrue)

			So(ms3.downloadSource, ShouldEqual, "groups/hgi/xxhash/0.8.1/singularity.sif")

			_, err = os.Stat(modulePath)
			So(err, ShouldBeNil)

			_, err = os.Stat(imagePath)
			So(err, ShouldBeNil)
			So(logWriter.String(), ShouldBeBlank)

			// TODO: test same def can't be built more than once simultaneously
		})

		Convey("Build returns an error if the upload fails", func() {
			ms3.fail = true
			err := builder.Build(def)
			So(err, ShouldNotBeNil)
		})

		Convey("Build logs an error if the run fails", func() {
			mwr.fail = true
			err := builder.Build(def)
			So(err, ShouldBeNil)

			<-mwr.ch

			ok := waitFor(func() bool {
				return logWriter.String() != ""
			})
			So(ok, ShouldBeTrue)

			So(logWriter.String(), ShouldContainSubstring,
				"msg=\"Async part of build failed\" err=\"Mock error\" s3Path=some_path/groups/hgi/xxhash/0.8.1")

			// TODO: the error log output from the run needs to be uploaded to
			// env repo.
		})

		// TODO: implement and test SpackLockToSoftPackYML and AddArtifactsToRepo
	})
}

func getExampleDefinition() *Definition {
	return &Definition{
		EnvironmentPath:    "groups/hgi/",
		EnvironmentName:    "xxhash",
		EnvironmentVersion: "0.8.1",
		Description:        "some help text",
		Packages: []Package{
			{
				Name:    "xxhash",
				Version: "0.8.1",
			},
			{
				Name:    "r-seurat",
				Version: "4",
			},
		},
	}
}

func waitFor(toRun func() bool) bool {
	timeout := time.NewTimer(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)

	defer ticker.Stop()
	defer timeout.Stop()

	for {
		select {
		case <-timeout.C:
			return false
		case <-ticker.C:
			if toRun() {
				return true
			}
		}
	}
}
