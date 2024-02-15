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

	graphQLEndpoint = "/graphql"
)

type gqlVariables struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Description string   `json:"description,omitempty"`
	Packages    Packages `json:"packages,omitempty"`
}

func newGQLVariables(fullPath, desc string, pkgs Packages) gqlVariables {
	return gqlVariables{
		Name:        filepath.Base(fullPath),
		Path:        filepath.Dir(fullPath),
		Description: desc,
		Packages:    pkgs,
	}
}

type gqlQuery struct {
	Variables gqlVariables `json:"variables"`
	Query     string       `json:"query"`
}

type EnvironmentResponse struct {
	TypeName string `json:"__typename"`
	Message  string `json:"message"`
}

type Response struct {
	Data struct {
		CreateEnvironment *EnvironmentResponse `json:"createEnvironment"`
		DeleteEnvironment *EnvironmentResponse `json:"deleteEnvironment"`
	} `json:"data"`
}

type Core struct {
	url string
}

func New(conf *config.Config) *Core {
	return &Core{
		url: strings.TrimSuffix(conf.CoreURL, "/") + graphQLEndpoint,
	}
}

func toJSON(thing any) io.Reader {
	var buf bytes.Buffer

	json.NewEncoder(&buf).Encode(thing) //nolint:errcheck

	return &buf
}

func (c *Core) doCoreRequest(graphQLPacket io.Reader) error {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		c.url,
		graphQLPacket,
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	return handleCoreResponse(resp)
}

func handleCoreResponse(resp *http.Response) error {
	var cr Response

	err := json.NewDecoder(resp.Body).Decode(&cr)
	if err != nil {
		return err
	}

	if cr.Data.DeleteEnvironment != nil {
		if cr.Data.DeleteEnvironment.TypeName != DeleteMutationSuccess {
			return internal.Error(cr.Data.DeleteEnvironment.Message)
		}
	}

	if cr.Data.CreateEnvironment != nil {
		if cr.Data.CreateEnvironment.TypeName != createMutationSuccess {
			return internal.Error(cr.Data.CreateEnvironment.Message)
		}
	}

	return nil
}
