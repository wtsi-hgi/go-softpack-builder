/*******************************************************************************
 * Copyright (c) 2023, 2024 Genome Research Ltd.
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
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/internal"
	"github.com/wtsi-hgi/go-softpack-builder/internal/core"
	"github.com/wtsi-hgi/go-softpack-builder/internal/gitmock"
	"github.com/wtsi-hgi/go-softpack-builder/wr"
)

const moduleLoadPrefix = "HGI/softpack"

type modifyRunner struct {
	cmd string
	*wr.Runner
	ch chan bool
}

func (m *modifyRunner) Add(_ string) (string, error) {
	jobID, err := m.Runner.Add(m.cmd)

	return jobID, err
}

func (m *modifyRunner) Wait(id string) (wr.WRJobStatus, error) {
	status, err := m.Runner.Wait(id)
	m.ch <- true

	return status, err
}

func (m *modifyRunner) Status(id string) (wr.WRJobStatus, error) {
	return m.Runner.Status(id)
}

func TestBuilder(t *testing.T) {
	Convey("Given binary cache and spack repo details and a Definition", t, func() {
		ms3 := &internal.MockS3{}
		mwr := internal.NewMockWR(1*time.Millisecond, 10*time.Millisecond)
		mc := &core.MockCore{Files: make(map[string]string)}
		msc := httptest.NewServer(mc)

		gm, commitHash := gitmock.New()

		gmhttp := httptest.NewServer(gm)

		var conf config.Config
		conf.S3.BinaryCache = "s3://spack"
		conf.S3.BuildBase = "some_path"
		conf.CustomSpackRepo = gmhttp.URL
		conf.CoreURL = msc.URL
		conf.Spack.BuildImage = "spack/ubuntu-jammy:v0.20.1"
		conf.Spack.FinalImage = "ubuntu:22.04"
		conf.Spack.ProcessorTarget = "x86_64_v4"

		builder, err := New(&conf, ms3, mwr)
		So(err, ShouldBeNil)

		var bcbCount atomic.Uint64
		bcb := func() {
			bcbCount.Add(1)
		}

		builder.SetPostBuildCallback(bcb)

		def := getExampleDefinition()

		Convey("You can generate a singularity .def", func() {
			defFile, err := builder.generateSingularityDef(def)

			So(err, ShouldBeNil)
			//nolint:lll
			So(defFile, ShouldEqual, `Bootstrap: docker
From: spack/ubuntu-jammy:v0.20.1
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
  - xxhash@0.8.1 arch=None-None-x86_64_v4
  - r-seurat@4 arch=None-None-x86_64_v4
  - py-anndata@3.14 arch=None-None-x86_64_v4
  view: /opt/view
  concretizer:
    unify: true
  config:
    install_tree: /opt/software
EOF

	# Install all the required software
	. /opt/spack/share/spack/setup-env.sh
	spack mirror add s3cache "s3://spack"
	tmpDir="$(mktemp -d)"
	git clone "`+gmhttp.URL+`" "$tmpDir"
	git -C "$tmpDir" checkout "`+commitHash+`"
	spack repo add "$tmpDir"
	spack buildcache keys --install --trust
	spack config add "config:install_tree:padded_length:128"
	spack -e . concretize
	spack -e . install --fail-fast || {
		spack -e . buildcache push -a s3cache
		false
	}
	spack -e . buildcache push -a s3cache
	spack gc -y
	spack env activate --sh -d . >> /opt/spack-environment/environment_modifications.sh

	# Strip the binaries to reduce the size of the image
	find -L /opt/view/* -type f -exec readlink -f '{}' \; | \
	xargs file -i | \
	grep 'charset=binary' | \
	grep 'x-executable\|x-archive\|x-sharedlib' | \
	awk -F: '{print $1}' | xargs strip

	exes="$(find $(grep "^export PATH=" /opt/spack-environment/environment_modifications.sh | sed -e 's/^export PATH=//' -e 's/;$//' | tr ":" "\n" | grep /opt/view | tr "\n" " ") -maxdepth 1 -executable -type l | xargs -r -L 1 readlink)"
	{
		for pkg in "xxhash" "r-seurat" "py-anndata"; do
			echo "$exes" | grep "/linux-[^/]*/gcc-[^/]*/$pkg-" || true
		done | xargs -L 1 -r basename
		echo "R"
		echo "Rscript"
		echo "python"
		find /opt/view/bin/ -maxdepth 1 -type f -executable | xargs -r -L 1 basename
	} | sort | uniq > executables

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

			repoURL := os.Getenv("GSB_TEST_REPO_URL")
			repoCommit := os.Getenv("GSB_TEST_REPO_COMMIT")

			if repoURL == "" || repoCommit == "" {
				SkipConvey("real test skipped, set GSB_TEST_REPO_URL and GSB_TEST_REPO_COMMIT to enable.", func() {})

				return
			}

			Convey("", func() {
				conf.CustomSpackRepo = repoURL
				expectedSubstring := "git clone \"" + repoURL + "\" \"$tmpDir\"\n" +
					"\tgit -C \"$tmpDir\" checkout \"" + repoCommit + "\""

				defFile, err := builder.generateSingularityDef(def)

				So(err, ShouldBeNil)
				So(defFile, ShouldContainSubstring, expectedSubstring)
			})
		})

		var logWriter internal.ConcurrentStringBuilder
		slog.SetDefault(slog.New(slog.NewTextHandler(&logWriter, nil)))

		Convey("You can do a Build", func() {
			conf.Module.ModuleInstallDir = t.TempDir()
			conf.Module.ScriptsInstallDir = t.TempDir()
			conf.Module.WrapperScript = "/path/to/wrapper"
			conf.Module.LoadPath = moduleLoadPrefix
			conf.Spack.ProcessorTarget = "x86_64_v4"
			ms3.Exes = "xxhsum\nxxh32sum\nxxh64sum\nxxh128sum\nR\nRscript\npython\n"
			err := builder.Build(def)
			So(err, ShouldBeNil)

			So(bcbCount.Load(), ShouldEqual, 0)

			So(ms3.Def, ShouldEqual, filepath.Join(def.getS3Path(), "singularity.def"))
			So(ms3.Data, ShouldContainSubstring, "specs:\n  - xxhash@0.8.1 arch=None-None-x86_64_v4\n"+
				"  - r-seurat@4 arch=None-None-x86_64_v4\n  - py-anndata@3.14 arch=None-None-x86_64_v4\n  view")

			mwr.SetRunning()
			<-mwr.Ch
			hash := fmt.Sprintf("%X", sha256.Sum256([]byte(ms3.Data)))
			So(mwr.Cmd, ShouldContainSubstring, "echo doing build with hash "+hash+"; sudo singularity build")

			modulePath := filepath.Join(conf.Module.ModuleInstallDir,
				def.EnvironmentPath, def.EnvironmentName, def.EnvironmentVersion)
			scriptsPath := filepath.Join(conf.Module.ScriptsInstallDir,
				def.EnvironmentPath, def.EnvironmentName, def.EnvironmentVersion+ScriptsDirSuffix)
			imagePath := filepath.Join(scriptsPath, core.ImageBasename)
			expectedExes := []string{"python", "R", "Rscript", "xxhsum", "xxh32sum", "xxh64sum", "xxh128sum"}

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

			info, err := os.Stat(modulePath)
			So(err, ShouldBeNil)

			perm := info.Mode().Perm()
			So(perm.String(), ShouldEqual, "-rwxr-xr-x")

			dir := modulePath
			for dir != conf.Module.ModuleInstallDir {
				info, err = os.Stat(dir)
				So(err, ShouldBeNil)
				perm = info.Mode().Perm()
				So(perm&0755, ShouldEqual, 0755)

				dir = filepath.Dir(dir)
			}

			dir = scriptsPath
			for dir != conf.Module.ScriptsInstallDir {
				info, err = os.Stat(dir)
				So(err, ShouldBeNil)
				perm = info.Mode().Perm()
				So(perm&0755, ShouldEqual, 0755)

				dir = filepath.Dir(dir)
			}

			info, err = os.Stat(imagePath)
			So(err, ShouldBeNil)

			perm = info.Mode().Perm()
			So(perm.String(), ShouldEqual, "-rwxr-xr-x")

			f, err := os.Open(imagePath)
			So(err, ShouldBeNil)
			imageData, err := io.ReadAll(f)
			So(err, ShouldBeNil)
			So(string(imageData), ShouldEqual, "image")

			So(logWriter.String(), ShouldBeBlank)

			expectedSoftpackYaml := `description: |
  some help text

  The following executables are added to your PATH:
    - xxhsum
    - xxh32sum
    - xxh64sum
    - xxh128sum
    - R
    - Rscript
    - python
packages:
  - xxhash@0.8.1
  - py-anndata@3.14
  - r-seurat@4
`

			expectedReadmeContent := "module load " + filepath.Join(moduleLoadPrefix, def.getS3Path())

			for file, expectedData := range map[string]string{
				core.SoftpackYaml:           expectedSoftpackYaml,
				core.ModuleForCoreBasename:  "module-whatis",
				core.SingularityDefBasename: "specs:\n  - xxhash@0.8.1 arch=None-None-x86_64_v4",
				core.SpackLockFile:          `"concrete_specs":`,
				core.BuilderOut:             "output",
				core.UsageBasename:          expectedReadmeContent,
			} {
				data, okg := mc.GetFile(filepath.Join(def.getRepoPath(), file))
				So(okg, ShouldBeTrue)
				So(data, ShouldContainSubstring, expectedData)
			}

			_, ok = mc.GetFile(filepath.Join(def.getRepoPath(), core.ImageBasename))
			So(ok, ShouldBeFalse)

			So(ms3.SoftpackYML, ShouldEqual, expectedSoftpackYaml)
			So(ms3.Readme, ShouldContainSubstring, expectedReadmeContent)

			So(bcbCount.Load(), ShouldEqual, 1)
		})

		Convey("Build returns an error if the upload fails", func() {
			ms3.Fail = true
			err := builder.Build(def)
			So(err, ShouldNotBeNil)

			So(bcbCount.Load(), ShouldEqual, 0)
		})

		Convey("Build logs an error if the run fails", func() {
			mwr.Fail = true
			err := builder.Build(def)
			So(err, ShouldBeNil)

			mwr.SetRunning()
			<-mwr.Ch

			ok := waitFor(func() bool {
				return logWriter.String() != ""
			})
			So(ok, ShouldBeTrue)

			So(logWriter.String(), ShouldContainSubstring,
				"msg=\"Async part of build failed\" err=\"Mock error\" s3Path=some_path/"+def.getS3Path())

			data, ok := mc.GetFile(filepath.Join(def.getRepoPath(), core.BuilderOut))
			So(ok, ShouldBeTrue)
			So(data, ShouldContainSubstring, "output")

			So(bcbCount.Load(), ShouldEqual, 1)
		})

		Convey("You can't run the same build simultaneously", func() {
			_, err := exec.LookPath("wr")
			if err != nil {
				SkipConvey("skipping a builder test since wr not in PATH", func() {})

				return
			}

			conf.Module.ModuleInstallDir = t.TempDir()
			conf.Module.ScriptsInstallDir = t.TempDir()
			conf.Module.WrapperScript = "/path/to/wrapper"
			conf.Module.LoadPath = moduleLoadPrefix
			ms3.Exes = "xxhsum\nxxh32sum\nxxh64sum\nxxh128sum\n"

			ch := make(chan bool, 1)
			mr := &modifyRunner{
				cmd:    "sleep 2s",
				Runner: wr.New("development"),
				ch:     ch,
			}

			builder.runner = mr

			err = builder.Build(def)
			So(err, ShouldBeNil)

			err = builder.Build(def)
			So(err, ShouldNotBeNil)
			So(err, ShouldEqual, ErrEnvironmentBuilding)

			<-ch
		})

		Convey("When the Core doesn't respond we get a meaningful error", func() {
			conf.CoreURL = "http://0.0.0.0:1234"
			conf.Module.ModuleInstallDir = t.TempDir()
			conf.Module.ScriptsInstallDir = t.TempDir()
			conf.Module.WrapperScript = "/path/to/wrapper"
			conf.Module.LoadPath = moduleLoadPrefix
			ms3.Exes = "xxhsum\nxxh32sum\nxxh64sum\nxxh128sum\n"

			err := builder.Build(def)
			So(err, ShouldBeNil)

			mwr.SetRunning()

			ok := waitFor(func() bool {
				return logWriter.String() != ""
			})
			So(ok, ShouldBeTrue)

			expectedLog := "Post \\\"http://0.0.0.0:1234" + uploadEndpoint + "?groups%2Fhgi%2Fxxhash-0.8.1\\\"" +
				": dial tcp 0.0.0.0:1234: connect: connection refused"
			So(logWriter.String(), ShouldContainSubstring, expectedLog)

			conf.CoreURL = msc.URL
			mc.Err = internal.Error("an error")

			logWriter.Reset()
			mwr.Ch = make(chan struct{})
			conf.Module.ModuleInstallDir = t.TempDir()
			conf.Module.ScriptsInstallDir = t.TempDir()

			err = builder.Build(def)
			So(err, ShouldBeNil)

			ok = waitFor(func() bool {
				return logWriter.String() != ""
			})
			So(ok, ShouldBeTrue)

			expectedLog = "\"Async part of build failed\" err=\"an error\\n\""

			So(logWriter.String(), ShouldContainSubstring, expectedLog)

			So(bcbCount.Load(), ShouldEqual, 2)
		})
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
			{
				Name:    "py-anndata",
				Version: "3.14",
			},
		},
	}
}

func (d *Definition) getS3Path() string {
	return filepath.Join(d.EnvironmentPath, d.EnvironmentName, d.EnvironmentVersion)
}

func (d *Definition) getRepoPath() string {
	return filepath.Join(d.EnvironmentPath, d.EnvironmentName) + "-" + d.EnvironmentVersion
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
