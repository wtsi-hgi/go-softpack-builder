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

const createMutationSuccess = "CreateEnvironmentSuccess"

const createMutationQuery = `
mutation ($name: String!, $description: String!, $path: String!, $packages: [PackageInput!]!) {
	createEnvironment(
		env: {name: $name, description: $description, path: $path, packages: $packages}
	) {
		__typename
		... on ` + createMutationSuccess + ` {
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

func newCreateMutation(path, description string, packages Packages) *gqlQuery {
	return &gqlQuery{
		Variables: newGQLVariables(path, description, packages),
		Query:     createMutationQuery,
	}
}

// Create contacts the core to schedule an environment build.
func (c *Core) Create(path, desc string, pkgs Packages) error {
	gq := newCreateMutation(path, desc, pkgs)

	return c.doGQLCoreRequest(toJSON(gq))
}
