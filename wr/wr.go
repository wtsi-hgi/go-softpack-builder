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
	"errors"
	"os/exec"
	"strings"
	"text/template"
)

//go:embed wr.tmpl
var wrTmplStr string
var wrTmpl *template.Template

func init() {
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
	cmd := exec.Command("wr", "add", "--deployment", r.deployment, "--sync", "--time", "8h", "--memory", "8G")
	cmd.Stdin = strings.NewReader(wrInput)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errStr := stderr.String()
		if errStr == "" {
			errStr = err.Error()
		}

		return errors.New(errStr)
	}

	return nil
}
