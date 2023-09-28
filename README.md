# go-softpack-builder (gsb)
Go implementation of a softpack builder service.

## Initial setup

You'll need an S3 bucket to be a binary cache, which needs GPG keys. Here's one
way this could be done:

```
cd /path/to
git clone --depth 1 -c feature.manyFiles=true https://github.com/spack/spack.git
source /software/hgi/installs/spack/share/spack/setup-env.sh
spack gpg create "user" "<user@domain>"
s3cmd put ~/spack/opt/spack/gpg/pubring.* s3://spack/
```

You'll also need a wr cloud deployment in OpenStack running an image with
singularity v3.10+ installed, to do singularity builds which need root.

With a wr config file such as /path/to/openstack/.wr_config.yml:
```
managerport: "46673"
managerweb: "46674"
managerhost: "hostname"
managerdir: "/path/to/openstack/.wr"
managerscheduler: "openstack"
cloudflavor: "^[mso].*$"
cloudflavorsets: "s4;m4;s2;m2;m1;o2"
clouddns: "172.18.255.1,172.18.255.2,172.18.255.3"
cloudos: "image-with-singularity"
clouduser: "ubuntu"
cloudram: 2048
clouddisk: 1
cloudconfigfiles: "~/.s3cfg,~/.aws/credentials,~/.aws/config,/path/to/spack/gpg/trustdb.gpg:~/spack/opt/spack/gpg/trustdb.gpg,/path/to/spack/gpg/pubring.kbx:~/spack/opt/spack/gpg/pubring.kbx,/path/to/spack/gpg/private-keys-v1.d/[keyname].key:~/spack/opt/spack/gpg/private-keys-v1.d/[keyname].key"
```

You can do the deploy like:

```
source ~/.openrc.sh
export WR_CONFIG_DIR=/path/to/openstack
wr cloud deploy
```

Now jobs submitted to this wr manager will run in OpenStack on a node where your
s3 credentials and gpg keys are copied to, and where singularity is installed,
enabling builds that use the binary cache.

Finally, you'll need go1.21+ in your PATH to install gsb:

```
git clone https://github.com/wtsi-hgi/go-softpack-builder.git
cd go-softpack-builder
make install
```

## Using gsb

Have a config file ~/.softpack/builder/gsb-config.yml that looks like this:

```
s3:
  binaryCache: "spack"
  buildBase: "spack/builds"

module:
  moduleInstallDir:  "/path/to/tcl_modules/softpack"
  scriptsInstallDir: "/different/path/for/images_and_scripts"
  loadPath: "softpack"
  dependencies:
    - "/path/to/modules/singularity/3.10.0"

customSpackRepo:
  url: "https://github.com/org/spack-repo.git"
  ref: "main"

spack:
  binaryCache: "https://binaries.spack.io/v0.20.1"
  buildImage: "spack/ubuntu-jammy:v0.20.1"
  finalImage: "ubuntu:22.04"
  processorTarget: "x86_64_v3"

coreURL: "http://x.y.z:9837/upload"
listenURL: "0.0.0.0:2456"
```

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
- loadPath is the base that users `module load`.
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
- listenURL is the address gsb will listen on for new build requests from core.

Start the builder service:

```
export WR_CONFIG_DIR=/path/to/openstack
gsb &
```

## Testing

Without a core service running, you can trigger a build by preparing a bash
script like this and running it while `gsb &` is running and your wr cloud
deployment is up:

```
#!/bin/bash

url="http://[listenURL]/environments/build";

curl -X POST -H "Content-Type: application/json" --data-binary @- "$url" <<HEREDOC
{
	"name": "users/user/test-build",
	"version": "1.0",
	"model": {
		"description": "A simple description",
		"packages": [{
			"name": "xxhash"
		}]
	}
}
HEREDOC
```

This should result in a job running in your wr manager that creates the
singularity image file and other artifacts in your S3 buildBase. The module
wrapper for the image will be installed to your installDir.

Only the last step, when gsb tries to send the artifacts to the core, will fail,
but you'll at least have a usable software installation of the environment that
can be tested and used.
