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
	"io"
	"path/filepath"

	"github.com/wtsi-hgi/go-softpack-builder/build"
)

const createMutationQuery = `
mutation ($name: String!, $description: String!, $path: String!, $packages: [PackageInput!]!) {
	createEnvironment(
		env: {name: $name, description: $description, path: $path, packages: $packages}
	) {
		__typename
		... on CreateEnvironmentSuccess {
			message
			__typename
		}
		... on Error {
			message
			__typename
		}
	}
}
`

type gqlVariables struct {
	Name        string         `json:"name"`
	Path        string         `json:"path"`
	Description string         `json:"description"`
	Packages    build.Packages `json:"packages"`
}

func newCreateMutation(path, description string, packages build.Packages) *gqlQuery {
	return &gqlQuery{
		Variables: gqlVariables{
			Name:        filepath.Base(path),
			Path:        filepath.Dir(path),
			Description: description,
			Packages:    packages,
		},
		Query: createMutationQuery,
	}
}

func (c *Core) Create(path, desc string, pkgs build.Packages) ([]byte, error) {
	gq := newCreateMutation(path, desc, pkgs)

	resp, err := c.doCoreRequest(toJSON(gq))
	if err != nil {
		return nil, err
	}

	return io.ReadAll(resp.Body)
}
