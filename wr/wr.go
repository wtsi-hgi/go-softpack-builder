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
	_ "embed"
	"os/exec"
	"strings"
	"text/template"
)

type Error struct {
	msg string
}

func (e Error) Error() string { return "wr add failed: " + e.msg }

//go:embed wr.tmpl
var wrTmplStr string
var wrTmpl *template.Template //nolint:gochecknoglobals

func init() { //nolint:gochecknoinits
	wrTmpl = template.Must(template.New("").Parse(wrTmplStr))
}

func SingularityBuildInS3WRInput(s3Path string) (string, error) {
	var w strings.Builder

	if err := wrTmpl.Execute(&w, s3Path); err != nil {
		return "", err
	}

	return w.String(), nil
}

type Runner struct {
	deployment string
}

func New(deployment string) *Runner {
	return &Runner{deployment: deployment}
}

func (r *Runner) Run(wrInput string) error {
	cmd := exec.Command("wr", "add", "--deployment", r.deployment, "--sync", //nolint:gosec
		"--time", "8h", "--memory", "8G", "--rerun")
	cmd.Stdin = strings.NewReader(wrInput)

	var std bytes.Buffer

	cmd.Stdout = &std
	cmd.Stderr = &std

	err := cmd.Run()
	if err != nil {
		errStr := std.String()
		if errStr == "" {
			errStr = err.Error()
		}

		return Error{msg: errStr}
	}

	return nil
}
