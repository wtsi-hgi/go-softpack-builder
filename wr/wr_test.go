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
		const hash = "0110"
		wrInput, err := SingularityBuildInS3WRInput(s3Path, hash)
		So(err, ShouldBeNil)
		So(wrInput, ShouldEqual, `{"cmd": "echo doing build with hash `+hash+`; `+
			`if sudo singularity build --bind $TMPDIR:/tmp $TMPDIR/singularity.sif singularity.def `+
			`&> $TMPDIR/builder.out; then `+
			`sudo singularity run $TMPDIR/singularity.sif cat /opt/spack-environment/executables > $TMPDIR/executables && `+
			`sudo singularity run $TMPDIR/singularity.sif cat /opt/spack-environment/spack.lock > $TMPDIR/spack.lock && `+
			`mv $TMPDIR/singularity.sif $TMPDIR/builder.out $TMPDIR/executables $TMPDIR/spack.lock .; `+
			`else mv $TMPDIR/builder.out .; mkdir logs; `+
			`sudo find $TMPDIR/root/spack-stage/ -maxdepth 2 -iname \"*.txt\" -exec cp {} logs/ \\; ; `+
			`false; fi", `+
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

	runner := New("development")
	runner.memory = "100M"
	runner.pollDuration = 10 * time.Millisecond

	Convey("You can run a cmd via wr", t, func() {
		now := time.Now()
		runArgs, repGrp := uniqueRunArgs("sleep 2s", "")
		jobID, err := runner.Add(runArgs)
		So(err, ShouldBeNil)
		err = runner.WaitForRunning(jobID)
		So(err, ShouldBeNil)
		status, err := runner.Wait(jobID)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, WRJobStatusComplete)
		So(time.Since(now), ShouldBeGreaterThan, 2*time.Second)

		statusOut := getWRStatus(runner.deployment, repGrp)
		So(statusOut, ShouldContainSubstring, `"State":"complete"`)

		jobID2, err := runner.Add(runArgs)
		So(err, ShouldBeNil)
		So(jobID2, ShouldEqual, jobID)
		err = runner.WaitForRunning(jobID)
		So(err, ShouldBeNil)
		status, err = runner.Wait(jobID)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, WRJobStatusComplete)

		runArgs, _ = uniqueRunArgs("false", "")
		jobID, err = runner.Add(runArgs)
		So(err, ShouldBeNil)
		err = runner.WaitForRunning(jobID)
		So(err, ShouldBeNil)
		status, err = runner.Wait(jobID)
		So(err, ShouldBeNil)
		So(status, ShouldEqual, WRJobStatusBuried)
	})

	Convey("WaitForRunning returns when the build starts running", t, func() {
		cmd := exec.Command("wr", "limit", "--deployment", runner.deployment, "-g", "limited:0") //nolint:gosec
		err := cmd.Run()
		So(err, ShouldBeNil)

		runArgs, _ := uniqueRunArgs("sleep 2s", "limited")
		jobID, err := runner.Add(runArgs)
		So(err, ShouldBeNil)

		runningCh := make(chan time.Time)
		errCh := make(chan error, 1)
		go func() {
			errCh <- runner.WaitForRunning(jobID)
			runningCh <- time.Now()
		}()

		<-time.After(100 * time.Millisecond)

		startTime := time.Now()
		cmd = exec.Command("wr", "limit", "--deployment", runner.deployment, "-g", "limited:1") //nolint:gosec
		err = cmd.Run()
		So(err, ShouldBeNil)

		status, err := runner.Wait(jobID)
		So(err, ShouldBeNil)
		endTime := time.Now()
		So(status, ShouldEqual, WRJobStatusComplete)

		So(<-errCh, ShouldBeNil)
		runningTime := <-runningCh
		So(runningTime, ShouldHappenAfter, startTime)
		So(runningTime, ShouldHappenBefore, endTime.Add(-1*time.Second))
	})
}

func uniqueRunArgs(cmd, limitGrp string) (string, string) {
	b := make([]byte, 16)
	n, err := rand.Read(b)
	So(n, ShouldEqual, 16)
	So(err, ShouldBeNil)

	repGrp := fmt.Sprintf("%x", b)

	return `{"cmd":"` + cmd + ` && echo ` + repGrp + `", "rep_grp": "` + repGrp +
		`", "limit_grps": ["` + limitGrp + `"], "retries": 0}`, repGrp
}

func getWRStatus(deployment, repGrp string) string {
	cmd := exec.Command("wr", "status", "--deployment", deployment, "-i", repGrp, "-o", "json")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	So(err, ShouldBeNil)
	So(stderr.String(), ShouldBeBlank)

	return stdout.String()
}
