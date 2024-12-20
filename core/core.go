/*******************************************************************************
 * Copyright (c) 2024 Genome Research Ltd.
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

package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/internal"
)

const (
	SingularityDefBasename = "singularity.def"
	ExesBasename           = "executables"
	SoftpackYaml           = "softpack.yml"
	SpackLockFile          = "spack.lock"
	BuilderOut             = "builder.out"
	ModuleForCoreBasename  = "module"
	UsageBasename          = "README.md"
	ImageBasename          = "singularity.sif"
	ErrNoCoreURL           = "no coreURL specified in config"
	ErrSomeResendsFailed   = "some queued environments failed to be resent from core to builder"

	resendEndpoint = "/resend-pending-builds"
	createEndpoint = "/create-environment"
	deleteEndpoint = "/delete-environment"
)

// EnvironmentResponse is the kind of return value we get from the core.
type EnvironmentResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// Core is used to interact with a real softpack-core service.
type Core struct {
	url string
}

// New creates a new Core struct, to contact the core via its configured URL.
func New(conf *config.Config) (*Core, error) {
	if conf == nil || conf.CoreURL == "" {
		return nil, internal.Error(ErrNoCoreURL)
	}

	return &Core{
		url: strings.TrimSuffix(conf.CoreURL, "/"),
	}, nil
}

func toJSON(thing any) io.Reader {
	var buf bytes.Buffer

	json.NewEncoder(&buf).Encode(thing) //nolint:errcheck

	return &buf
}

func (c *Core) doCoreRequest(endpoint string, content io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		c.url+endpoint,
		content,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}

// ResendResponse is the response that the core service sends when a resend
// request is sent to it.
type ResendResponse struct {
	Message   string
	Successes int
	Failures  int
}

// FullSuccess returns true if there were no failures.
func (rr *ResendResponse) FullSuccess() bool {
	return rr.Failures == 0
}

// ResendPendingBuilds posts to core's resend-pending-builds endpoint and
// returns an error if the post failed, or core did not respond with a full
// success.
func (c *Core) ResendPendingBuilds() error {
	resp, err := c.doCoreRequest(resendEndpoint, strings.NewReader(""))
	if err != nil {
		return err
	}

	var rr ResendResponse

	err = json.NewDecoder(resp.Body).Decode(&rr)
	if err != nil {
		return err
	}

	if !rr.FullSuccess() {
		return internal.Error(ErrSomeResendsFailed)
	}

	return nil
}

type environmentInput struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Description string   `json:"description"`
	Packages    Packages `json:"packages"`
}

// Create contacts the core to schedule an environment build.
func (c *Core) Create(path, desc string, pkgs Packages) error {
	return handleResponse(c.doCoreRequest(createEndpoint, toJSON(environmentInput{
		Name:        filepath.Base(path),
		Path:        filepath.Dir(path),
		Description: desc,
		Packages:    pkgs,
	})))
}

func handleResponse(resp *http.Response, err error) error {
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	var r EnvironmentResponse

	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return err
	}

	if r.Error != "" {
		return errors.New(r.Error) //nolint:goerr113
	}

	return nil
}

type deleteEnvironmentInput struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Delete contacts the core to delete an environment.
func (c *Core) Delete(path string) error {
	return handleResponse(c.doCoreRequest(deleteEndpoint, toJSON(deleteEnvironmentInput{
		Name: filepath.Base(path),
		Path: filepath.Dir(path),
	})))
}
