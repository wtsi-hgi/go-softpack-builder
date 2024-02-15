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
	"bufio"
	"bytes"
	_ "embed"
	"log/slog"
	"os/exec"
	"strings"
	"text/template"
	"time"
)

type WRJobStatus int

const (
	WRJobStatusInvalid WRJobStatus = iota
	WRJobStatusDelayed
	WRJobStatusReady
	WRJobStatusReserved
	WRJobStatusRunning
	WRJobStatusLost
	WRJobStatusBuried
	WRJobStatusComplete
)

const (
	plainStatusCols     = 2
	defaultPollDuration = 5 * time.Second
)

type Error struct {
	msg string
}

func (e Error) Error() string { return "wr cmd failed: " + e.msg }

//go:embed wr.tmpl
var wrTmplStr string
var wrTmpl *template.Template //nolint:gochecknoglobals

func init() { //nolint:gochecknoinits
	wrTmpl = template.Must(template.New("").Parse(wrTmplStr))
}

// SingularityBuildInS3WRInput returns wr input that could be piped to `wr add`
// and that would run a singularity build where the working directory is a fuse
// mount of the given s3Path.
func SingularityBuildInS3WRInput(s3Path, hash string) (string, error) {
	var w strings.Builder

	if err := wrTmpl.Execute(&w, struct {
		S3Path, Hash string
	}{
		s3Path,
		hash,
	}); err != nil {
		return "", err
	}

	return w.String(), nil
}

// Runner lets you Run() a wr add command.
type Runner struct {
	deployment   string
	memory       string
	pollDuration time.Duration
}

// New returns a Runner that will use the given wr deployment to wr add jobs
// during Run().
func New(deployment string) *Runner {
	return &Runner{
		deployment:   deployment,
		memory:       "43G",
		pollDuration: defaultPollDuration,
	}
}

// Run pipes the given wrInput (eg. as produced by
// SingularityBuildInS3WRInput()) to `wr add`, which adds a job to wr's queue
// and returns its ID. You should call Wait(ID) to actually wait for the job to
// finishing running.
//
// The memory defaults to 8GB, time to 8hrs, and if the cmd in the input has
// previously been run, the cmd will be re-run.
//
// NB: if the cmd is a duplicate of a currently queued job, this will not
// generate an error, but just return the id of the existing job.
func (r *Runner) Add(wrInput string) (string, error) {
	cmd := exec.Command("wr", "add", "--deployment", r.deployment, "--simple", //nolint:gosec
		"--time", "8h", "--memory", r.memory, "-o", "2", "--rerun")
	cmd.Stdin = strings.NewReader(wrInput)

	return r.runWRCmd(cmd)
}

func (r *Runner) runWRCmd(cmd *exec.Cmd) (string, error) {
	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	slog.Debug("ran wr command", "cmd", cmd.String(), "stdout", stdout.String(),
		"stderr", stderr.String(), "err", err, "exitcode", cmd.ProcessState.ExitCode())

	if err != nil {
		errStr := stderr.String()
		if !strings.Contains(errStr, "EROR") {
			return strings.TrimSpace(stdout.String()), nil
		}

		if errStr == "" {
			errStr = err.Error()
		}

		return "", Error{msg: errStr}
	}

	return strings.TrimSpace(stdout.String()), nil
}

// WaitForRunning waits until the given wr job either starts running, or exits.
func (r *Runner) WaitForRunning(id string) error {
	var err error

	cb := func(status WRJobStatus, cbErr error) bool {
		err = cbErr

		return err != nil || statusIsStarted(status) || statusIsExited(status)
	}

	r.pollStatus(id, cb)

	return err
}

func statusIsStarted(status WRJobStatus) bool {
	return status == WRJobStatusRunning || status == WRJobStatusLost
}

func statusIsExited(status WRJobStatus) bool {
	return status == WRJobStatusInvalid || status == WRJobStatusBuried || status == WRJobStatusComplete
}

// pollStatusCallback receives a WRJobStatus and error, and should return true
// if you want to stop polling now.
type pollStatusCallback = func(WRJobStatus, error) bool

func (r *Runner) pollStatus(id string, cb pollStatusCallback) {
	ticker := time.NewTicker(r.pollDuration)
	defer ticker.Stop()

	for range ticker.C {
		if cb(r.Status(id)) {
			return
		}
	}
}

// Wait waits for the given wr job to exit.
func (r *Runner) Wait(id string) (WRJobStatus, error) {
	var (
		status WRJobStatus
		err    error
	)

	cb := func(cbStatus WRJobStatus, cbErr error) bool {
		status = cbStatus
		err = cbErr

		return err != nil || statusIsExited(status)
	}

	r.pollStatus(id, cb)

	return status, err
}

// Status returns the status of the wr job with the given internal ID.
//
// Returns WRJobStatusInvalid if the ID wasn't found. Returns WRJobStatusBuried
// if it failed. Only returns an error if there was a problem getting the
// status.
func (r *Runner) Status(id string) (WRJobStatus, error) {
	cmd := exec.Command("wr", "status", "--deployment", r.deployment, "-o", //nolint:gosec
		"plain", "-i", id, "-y")

	out, err := r.runWRCmd(cmd)
	if err != nil {
		slog.Error("wr status command failed", "err", err)

		return WRJobStatusInvalid, err
	}

	return parseWRStatus(out, id)
}

func parseWRStatus(wrStatusOutput, id string) (WRJobStatus, error) {
	scanner := bufio.NewScanner(strings.NewReader(wrStatusOutput))
	for scanner.Scan() {
		cols := strings.Split(scanner.Text(), "\t")
		if len(cols) != plainStatusCols {
			continue
		}

		if cols[0] != id {
			continue
		}

		return statusStringToType(cols[1]), nil
	}

	slog.Error("wr status parsing to find a job failed", "id", id, "err", scanner.Err())

	return WRJobStatusInvalid, scanner.Err()
}

func statusStringToType(status string) WRJobStatus { //nolint:gocyclo
	switch status {
	case "delayed":
		return WRJobStatusDelayed
	case "ready":
		return WRJobStatusReady
	case "reserved":
		return WRJobStatusReserved
	case "running":
		return WRJobStatusRunning
	case "lost":
		return WRJobStatusLost
	case "buried":
		return WRJobStatusBuried
	case "complete":
		return WRJobStatusComplete
	default:
		return WRJobStatusInvalid
	}
}

func (r *Runner) Cleanup() error {
	cmd := exec.Command("wr", "remove", "--deployment", r.deployment, "-a") //nolint:gosec

	_, err := r.runWRCmd(cmd)

	return err
}
