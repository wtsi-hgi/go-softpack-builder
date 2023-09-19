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

package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/wtsi-hgi/go-softpack-builder/spack"
)

type mockBuilder struct {
	received *spack.Definition
}

func (m *mockBuilder) Build(def *spack.Definition) error {
	m.received = def
	return nil
}

func TestServer(t *testing.T) {
	Convey("Given a mock Builder and test http server, core posts result in a Definition being sent to Build()", t, func() {
		mb := new(mockBuilder)

		handler := New(mb)
		server := httptest.NewServer(handler)
		So(server, ShouldNotBeNil)

		resp, err := server.Client().Post(server.URL+"/environments/build", "application/json",
			strings.NewReader(`
{
	"name": "users/user/myenv",
	"version": "0.8.1",
	"model": {
		"description": "help text",
		"packages": [{"name": "xxhash", "version": "0.8.1"}]
	}
}
`))
		So(err, ShouldBeNil)
		So(resp.StatusCode, ShouldEqual, http.StatusOK)
		So(mb.received, ShouldResemble, &spack.Definition{
			EnvironmentPath:    "users/user/",
			EnvironmentName:    "myenv",
			EnvironmentVersion: "0.8.1",
			Description:        "help text",
			Packages: []spack.Package{
				{
					Name:    "xxhash",
					Version: "0.8.1",
				},
			},
		})
	})
}
