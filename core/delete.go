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

const DeleteMutationSuccess = "DeleteEnvironmentSuccess"

const deleteMutationQuery = `
mutation ($name: String!, $path: String!) {
	deleteEnvironment(
		name: $name
		path: $path
	) {
		__typename
		... on ` + DeleteMutationSuccess + ` {
			message
		}
		... on EnvironmentNotFoundError {
			message
			path
			name
		}
	}
}
`

func newDeleteMutation(path string) *gqlQuery {
	return &gqlQuery{
		Variables: newGQLVariables(path, "", nil),
		Query:     deleteMutationQuery,
	}
}

// Delete contacts the core to delete an environment.
func (c *Core) Delete(path string) error {
	gq := newDeleteMutation(path)

	return c.doGQLCoreRequest(toJSON(gq))
}
