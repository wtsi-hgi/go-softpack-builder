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

package cmd

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wtsi-hgi/go-softpack-builder/build"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/server"
)

// global options.
var configPath string
var debug bool

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a build service",
	Long: `Start a build service.

The build service is intended to be called from Softpack Core
(https://github.com/wtsi-hgi/softpack-core), but can also be called by using the
build subcommand.

On receiving a request service will trigger the build of the desired software
and install a module for it.

Requires a wr manager that is capable of running commands as root.

Have a config file ~/.softpack/builder/gsb-config.yml that looks like this:

---

s3:
  binaryCache: "spack"
  buildBase: "spack/builds"

module:
  moduleInstallDir:  "/path/to/tcl_modules/softpack"
  scriptsInstallDir: "/different/path/for/images_and_scripts"
  loadPath: "softpack"
  dependencies:
    - "/path/to/modules/singularity/3.10.0"

customSpackRepo: "https://github.com/org/spack-repo.git"

spack:
  binaryCache: "https://binaries.spack.io/v0.20.1"
  buildImage: "spack/ubuntu-jammy:v0.20.1"
  finalImage: "ubuntu:22.04"
  processorTarget: "x86_64_v3"

coreURL: "http://x.y.z:9837/upload"
listenURL: "0.0.0.0:2456"

---

Where:

- s3.binaryCache is the name of your S3 bucket that will be used as a Spack
  binary cache and has the gpg files copied to it.
- buildBase is the bucket and optional sub "directory" that builds will occur
  in.
- moduleInstallDir is the absolute base path that modules will be installed to
  following a build. This directory needs to be accessible by your users.
  Directories and files that gsb creates within will be world readable and
  executable.
- scriptsInstallDir is like moduleInstallDir, but will contain the images and
  wrapper script symlinks for your builds. These are kept separately from the
  tcl module files, because having large files alongside the tcl file will slow
  down the module system.
- loadPath is the base that users "module load".
- dependencies are any module dependencies that need to be loaded because that
  software won't be part of the environments being built. Users will at least
  need singularity, since the modules created by softpack run singularity
  images.
- customSpackRepo is your own repository of Spack packages containing your own
  custom recipies. It will be used in addition to Spack's build-in repo during
  builds.
- spack.binaryCache is the URL of spack's binary cache. The version should match
  the spack version in your buildImage. You can find the URLs via
  https://cache.spack.io.
- buildImage is spack's docker image from their docker hub with the desired
  version (don't use latest if you want reproducability) of spack and desired
  OS.
- finalImage is the base image for the OS you want the software spack builds to
  installed inside (it should be the same OS as buildImage).
- processorTarget should match the lowest common denominator CPU for the
  machines where builds will be used. For example, x86_64_v3.
- coreURL is the URL of a running softpack core service, that will be used to
  send build artifacts to so that it can store them in a softpack environements
  git repository and make them visible on the softpack frontend.
- listenURL is the address gsb will listen on for new build requests from core.`,
	Run: func(_ *cobra.Command, _ []string) {
		if debug {
			h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			})
			slog.SetDefault(slog.New(h))
		}

		conf := getConfig()

		b, err := build.New(conf)
		if err != nil {
			die("could not create a builder: %s", err)
		}

		err = http.ListenAndServe(conf.ListenURL, server.New(b)) //nolint:gosec
		if err != nil {
			die("could not start server: %s", err)
		}
	},
}

func init() {
	serverCmd.Flags().StringVar(&configPath, "config", "",
		"path to config file")
	serverCmd.Flags().BoolVar(&debug, "debug", false,
		"turn on debug logging output")
}

func getConfig() *config.Config {
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			die("could not get home dir: %s", err)
		}

		configPath = filepath.Join(home, ".softpack", "builder", "gsb-config.yml")
	}

	f, err := os.Open(configPath)
	if err != nil {
		die("could not open config file: %s", err)
	}

	conf, err := config.Parse(f)
	f.Close()

	if err != nil {
		die("could not parse config file: %s", err)
	}

	return conf
}
