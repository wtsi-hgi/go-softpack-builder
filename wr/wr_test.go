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

package wr

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestWR(t *testing.T) {
	// buildBase would come from our yml config, and envPath and Name
	// would come from the Definition from the core post to our Server.
	buildBase := "spack/builds/"
	envPath := "users/user"
	envName := "myenv"
	s3Path := buildBase + envPath + "/" + envName

	Convey("You can generate a wr input", t, func() {
		wrInput, err := SingularityBuildInS3WRInput(s3Path)
		So(err, ShouldBeNil)
		So(wrInput, ShouldEqual, `{"cmd": "echo doing build in spack/builds/users/user/myenv; `+
			`sudo singularity build $TMPDIR/singularity.sif singularity.def &> builder.out && `+
			`sudo singularity run $TMPDIR/singularity.sif cat /opt/spack-environment/executables > executables && `+
			`sudo singularity run $TMPDIR/singularity.sif cat /opt/spack-environment/spack.lock > spack.lock && `+
			`mv $TMPDIR/singularity.sif .", `+
			`"retries": 0, "rep_grp": "singularity_build-spack/builds/users/user/myenv", "limit_grps": ["s3cache"], `+
			`"mounts": [{"Targets": [{"Path":"spack/builds/users/user/myenv","Write":true,"Cache":true}]}]}`)

		var m map[string]any
		err = json.NewDecoder(strings.NewReader(wrInput)).Decode(&m)
		So(err, ShouldBeNil)
	})

	gsbWR := os.Getenv("GSB_WR_TEST")
	if gsbWR == "" {
		SkipConvey("Skipping WR run test, set GSB_WR_TEST to enable", t, func() {})

		return
	}

	Convey("You can run a WR command", t, func() {
		now := time.Now()

		runner := New("development")

		runArgs, repGrp := uniqueRunArgs("sleep 2s")
		err := runner.Run(runArgs)
		So(err, ShouldBeNil)
		So(time.Since(now), ShouldBeGreaterThan, 2*time.Second)

		cmd := exec.Command("wr", "status", "--deployment", runner.deployment, "-i", repGrp, "-o", "json") //nolint:gosec
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err = cmd.Run()
		So(err, ShouldBeNil)
		So(stderr.String(), ShouldBeBlank)
		So(stdout.String(), ShouldContainSubstring, `"State":"complete"`)

		err = runner.Run(runArgs)
		So(err, ShouldBeNil)

		runArgs, _ = uniqueRunArgs("false")
		err = runner.Run(runArgs)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "wr add failed: exit status 1")
	})

	Convey("SingularityBuildInS3WRInput is valid wr input", t, func() {

	})
}

func uniqueRunArgs(cmd string) (string, string) {
	b := make([]byte, 16)
	n, err := rand.Read(b)
	So(n, ShouldEqual, 16)
	So(err, ShouldBeNil)

	repGrp := fmt.Sprintf("%x", b)

	return `{"cmd":"` + cmd + ` && echo ` + repGrp + `", "rep_grp": "` + repGrp + `", "retries": 0}`, repGrp
}
