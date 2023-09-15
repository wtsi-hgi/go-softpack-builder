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
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBuilder(t *testing.T) {
	Convey("Given binary cache and spack repo details and a Definition", t, func() {
		builder := New("s3://spack", "https://github.com/spack/repo", "some_tag")
		So(builder, ShouldNotBeNil)

		Convey("You can generate a singularity .def", func() {
			def, err := builder.GenerateSingularityDef(getExampleDefinition())

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
	- xxhash@0.8.1
	- r-seurat@4
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
				Exe:     "xxhsum",
			},
			{
				Name:    "r-seurat",
				Version: "4",
			},
		},
	}
}
