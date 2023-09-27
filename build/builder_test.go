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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/wr"
)

const ErrMock = Error("Mock error")
const moduleLoadPrefix = "HGI/softpack"

type mockS3 struct {
	ch             chan struct{}
	data           string
	dest           string
	downloadSource string
	fail           bool
	exes           string
	mu             sync.Mutex
}

func (m *mockS3) UploadData(data io.Reader, dest string) error {
	defer close(m.ch)

	if m.fail {
		return ErrMock
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
	m.mu.Lock()
	defer m.mu.Unlock()

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

	if filepath.Base(source) == builderOut {
		return io.NopCloser(strings.NewReader("output")), nil
	}

	if filepath.Base(source) == spackLock {
		return io.NopCloser(strings.NewReader(`{"_meta":{"file-type":"spack-lockfile","lockfile-version":5,"specfile-version":4},"spack":{"version":"0.21.0.dev0","type":"git","commit":"dac3b453879439fd733b03d0106cc6fe070f71f6"},"roots":[{"hash":"oibd5a4hphfkgshqiav4fdkvw4hsq4ek","spec":"xxhash arch=None-None-x86_64_v3"}, {"hash":"1ibd5a4hphfkgshqiav4fdkvw4hsq4e1","spec":"py-anndata arch=None-None-x86_64_v3"}, {"hash":"2ibd5a4hphfkgshqiav4fdkvw4hsq4e2","spec":"r-seurat arch=None-None-x86_64_v3"}],"concrete_specs":{"oibd5a4hphfkgshqiav4fdkvw4hsq4ek":{"name":"xxhash","version":"0.8.1","arch":{"platform":"linux","platform_os":"ubuntu22.04","target":"x86_64_v3"},"compiler":{"name":"gcc","version":"11.4.0"},"namespace":"builtin","parameters":{"build_system":"makefile","cflags":[],"cppflags":[],"cxxflags":[],"fflags":[],"ldflags":[],"ldlibs":[]},"package_hash":"wuj5b2kjnmrzhtjszqovcvgc3q46m6hoehmiccimi5fs7nmsw22a====","hash":"oibd5a4hphfkgshqiav4fdkvw4hsq4ek"},"2ibd5a4hphfkgshqiav4fdkvw4hsq4e2":{"name":"r-seurat","version":"4","arch":{"platform":"linux","platform_os":"ubuntu22.04","target":"x86_64_v3"},"compiler":{"name":"gcc","version":"11.4.0"},"namespace":"builtin","parameters":{"build_system":"makefile","cflags":[],"cppflags":[],"cxxflags":[],"fflags":[],"ldflags":[],"ldlibs":[]},"package_hash":"2uj5b2kjnmrzhtjszqovcvgc3q46m6hoehmiccimi5fs7nmsw222====","hash":"2ibd5a4hphfkgshqiav4fdkvw4hsq4e2"}, "1ibd5a4hphfkgshqiav4fdkvw4hsq4e1":{"name":"py-anndata","version":"3.14","arch":{"platform":"linux","platform_os":"ubuntu22.04","target":"x86_64_v3"},"compiler":{"name":"gcc","version":"11.4.0"},"namespace":"builtin","parameters":{"build_system":"makefile","cflags":[],"cppflags":[],"cxxflags":[],"fflags":[],"ldflags":[],"ldlibs":[]},"package_hash":"2uj5b2kjnmrzhtjszqovcvgc3q46m6hoehmiccimi5fs7nmsw222====","hash":"1ibd5a4hphfkgshqiav4fdkvw4hsq4e1"}}}`)), nil //nolint:lll
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
		return ErrMock
	}

	m.cmd = cmd

	return nil
}

type modifyRunner struct {
	cmd string
	*wr.Runner
	ch chan bool
}

func (m *modifyRunner) Run(_ string) error {
	err := m.Runner.Run(m.cmd)
	m.ch <- true

	return err
}

type mockCore struct {
	mu    sync.RWMutex
	files map[string]string
}

func (m *mockCore) setFile(filename, contents string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.files[filename] = contents
}

func (m *mockCore) getFile(filename string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contents, ok := m.files[filename]

	return contents, ok
}

func (m *mockCore) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	mr, err := r.MultipartReader()
	if err != nil {
		return
	}

	envPath, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		return
	}

	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return
		}

		name := p.FileName()

		buf, err := io.ReadAll(p)
		if err != nil {
			return
		}

		m.setFile(filepath.Join(envPath, name), string(buf))
	}
}

type concurrentStringBuilder struct {
	mu sync.RWMutex
	strings.Builder
}

func (c *concurrentStringBuilder) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Builder.Write(p)
}

func (c *concurrentStringBuilder) WriteString(str string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Builder.WriteString(str)
}

func (c *concurrentStringBuilder) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Builder.String()
}

func TestBuilder(t *testing.T) {
	Convey("Given binary cache and spack repo details and a Definition", t, func() {
		ms3 := &mockS3{ch: make(chan struct{})}
		mwr := &mockWR{ch: make(chan struct{})}
		mc := &mockCore{files: make(map[string]string)}
		msc := httptest.NewServer(mc)

		var conf config.Config
		conf.S3.BinaryCache = "s3://spack"
		conf.S3.BuildBase = "some_path"
		conf.CustomSpackRepo.URL = "https://github.com/spack/repo"
		conf.CustomSpackRepo.Ref = "some_tag"
		conf.CoreURL = msc.URL
		conf.Spack.BinaryCache = "https://binaries.spack.io/v0.20.1"
		conf.Spack.BuildImage = "spack/ubuntu-jammy:v0.20.1"
		conf.Spack.FinalImage = "ubuntu:22.04"

		builder := &Builder{
			config:              &conf,
			s3:                  ms3,
			runner:              mwr,
			runningEnvironments: make(map[string]bool),
		}

		def := getExampleDefinition()

		Convey("You can generate a singularity .def", func() {
			def, err := builder.generateSingularityDef(def)

			So(err, ShouldBeNil)
			//nolint:lll
			So(def, ShouldEqual, `Bootstrap: docker
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
  - xxhash@0.8.1 arch=None-None-x86_64_v3
  - r-seurat@4 arch=None-None-x86_64_v3
  - py-anndata@3.14 arch=None-None-x86_64_v3
  view: /opt/view
  concretizer:
    unify: true
  config:
    install_tree: /opt/software
EOF

	# Install all the required software
	. /opt/spack/share/spack/setup-env.sh
	spack mirror add spackCache https://binaries.spack.io/v0.20.1
	spack mirror add s3cache "s3://spack"
	tmpDir="$(mktemp -d)"
	git clone "https://github.com/spack/repo" "$tmpDir"
	git -C "$tmpDir" checkout "some_tag"
	spack repo add "$tmpDir"
	spack buildcache keys --install --trust
	spack config add "config:install_tree:padded_length:128"
	spack -e . concretize
	spack -e . install
	spack -e . buildcache push -a --rebuild-index -f s3cache
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
	{
		for pkg in "xxhash" "r-seurat" "py-anndata"; do
			echo "$exes" | grep "/linux-[^/]*/gcc-[^/]*/$pkg-";
		done | xargs -L 1 basename;
		echo "R";
		echo "Rscript";
		echo "python";
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
		})

		var logWriter concurrentStringBuilder
		slog.SetDefault(slog.New(slog.NewTextHandler(&logWriter, nil)))

		Convey("You can do a Build", func() {
			conf.Module.InstallDir = t.TempDir()
			conf.Module.WrapperScript = "/path/to/wrapper"
			conf.Module.LoadPath = moduleLoadPrefix
			ms3.exes = "xxhsum\nxxh32sum\nxxh64sum\nxxh128sum\nR\nRscript\npython\n"
			err := builder.Build(def)
			So(err, ShouldBeNil)

			<-ms3.ch
			So(ms3.dest, ShouldEqual, "groups/hgi/xxhash/0.8.1/singularity.def")
			So(ms3.data, ShouldContainSubstring, "specs:\n  - xxhash@0.8.1 arch=None-None-x86_64_v3\n"+
				"  - r-seurat@4 arch=None-None-x86_64_v3\n  - py-anndata@3.14 arch=None-None-x86_64_v3\n  view")

			<-mwr.ch
			So(mwr.cmd, ShouldContainSubstring, "echo doing build in some_path/groups/hgi/xxhash/0.8.1; sudo singularity build")

			envPath := filepath.Join(conf.Module.InstallDir,
				def.EnvironmentPath, def.EnvironmentName)

			modulePath := filepath.Join(envPath, def.EnvironmentVersion)
			scriptsPath := filepath.Join(envPath, def.EnvironmentVersion+scriptsDirSuffix)
			imagePath := filepath.Join(scriptsPath, imageBasename)
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

			ms3.mu.Lock()
			So(ms3.downloadSource, ShouldEqual, "groups/hgi/xxhash/0.8.1/singularity.sif")
			ms3.mu.Unlock()

			info, err := os.Stat(modulePath)
			So(err, ShouldBeNil)

			perm := info.Mode().Perm()
			So(perm.String(), ShouldEqual, "-rwxr-xr-x")

			dir := scriptsPath
			for dir != conf.Module.InstallDir {
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

			So(logWriter.String(), ShouldBeBlank)

			for file, expectedData := range map[string]string{
				softpackYaml: `description: |
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
`,
				moduleForCoreBasename:  "module-whatis",
				singularityDefBasename: "specs:\n  - xxhash@0.8.1 arch=None-None-x86_64_v3",
				spackLock:              `"concrete_specs":`,
				imageBasename:          "mock",
				builderOut:             "output",
				usageBasename:          "module load " + moduleLoadPrefix + "/groups/hgi/xxhash/0.8.1",
			} {
				data, ok := mc.getFile("groups/hgi/xxhash-0.8.1/" + file)
				So(ok, ShouldBeTrue)
				So(data, ShouldContainSubstring, expectedData)
			}
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

			data, ok := mc.getFile("groups/hgi/xxhash-0.8.1/" + builderOut)
			So(ok, ShouldBeTrue)
			So(data, ShouldContainSubstring, "output")
		})

		Convey("You can't run the same build simultaneously", func() {
			conf.Module.InstallDir = t.TempDir()
			conf.Module.WrapperScript = "/path/to/wrapper"
			conf.Module.LoadPath = moduleLoadPrefix
			ms3.exes = "xxhsum\nxxh32sum\nxxh64sum\nxxh128sum\n"

			mr := &modifyRunner{
				cmd:    "sleep 2s",
				Runner: wr.New("development"),
				ch:     make(chan bool, 1),
			}

			builder.runner = mr

			err := builder.Build(def)
			So(err, ShouldBeNil)

			err = builder.Build(def)
			So(err, ShouldNotBeNil)
			So(err, ShouldEqual, ErrEnvironmentBuilding)
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
