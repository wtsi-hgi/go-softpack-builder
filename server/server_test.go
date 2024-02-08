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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/build"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/internal"
	"github.com/wtsi-hgi/go-softpack-builder/internal/core"
	"github.com/wtsi-hgi/go-softpack-builder/internal/gitmock"
)

type mockBuilder struct {
	received  []*build.Definition
	requested []time.Time
}

func (m *mockBuilder) Build(def *build.Definition) error {
	m.received = append(m.received, def)

	return nil
}

func (m *mockBuilder) Status() []build.Status {
	statuses := make([]build.Status, len(m.received))

	for i, def := range m.received {
		statuses[i] = build.Status{
			Name:      filepath.Join(def.EnvironmentPath, def.EnvironmentName) + "-" + def.EnvironmentVersion,
			Requested: m.requested[i],
		}
	}

	return statuses
}

func TestServerMock(t *testing.T) {
	Convey("Posts to core result in a Definition being sent to Build()", t, func() {
		mb := new(mockBuilder)

		handler := New(mb)
		server := httptest.NewServer(handler)

		resp, err := server.Client().Post(server.URL+"/environments/build", "application/json", //nolint:noctx
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
		So(mb.received[0], ShouldResemble, &build.Definition{
			EnvironmentPath:    "users/user/",
			EnvironmentName:    "myenv",
			EnvironmentVersion: "0.8.1",
			Description:        "help text",
			Packages: []build.Package{
				{
					Name:    "xxhash",
					Version: "0.8.1",
				},
			},
		})

		Convey("Unless the request is invalid", func() {
			for _, test := range [...]struct {
				InputJSON   string
				OutputError string
			}{
				{
					InputJSON: `
						{
							"name": "myenv",
							"version": "0.8.1",
							"model": {
								"description": "help text",
								"packages": [{"name": "xxhash", "version": "0.8.1"}]
							}
						}`,
					OutputError: "error validating request: invalid environment path\n",
				},
				{
					InputJSON: `
					{
						"name": "groups/hgi/myenv",
						"model": {
							"description": "help text",
							"packages": [{"name": "xxhash", "version": "0.8.1"}]
						}
					}`,
					OutputError: "error validating request: environment version required\n",
				},
				{
					InputJSON: `
					{
						"name": "groups/hgi/myenv",
						"version": "0.8.1",
						"model": {
							"description": "help text"
						}
					}`,
					OutputError: "error validating request: packages required\n",
				},
				{
					InputJSON: `
					{
						"name": "groups/hgi/myenv",
						"version": "0.8.1",
						"model": {
							"description": "help text",
							"packages": [{"version": "0.8.1"}]
						}
					}`,
					OutputError: "error validating request: package names required\n",
				},
			} {
				resp, err = server.Client().Post(server.URL+"/environments/build", "application/json", //nolint:noctx
					strings.NewReader(test.InputJSON))

				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, http.StatusBadRequest)
				body, erra := io.ReadAll(resp.Body)
				So(erra, ShouldBeNil)
				So(string(body), ShouldEqual, test.OutputError)
			}
		})

		Convey("After which you can get the queued/building/built status for it", func() {
			mb.requested = append(mb.requested, time.Now())
			resp, err := server.Client().Get(server.URL + "/environments/status") //nolint:noctx
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			var statuses []build.Status
			err = json.NewDecoder(resp.Body).Decode(&statuses)
			So(err, ShouldBeNil)
			So(len(statuses), ShouldEqual, 1)
			So(statuses[0].Name, ShouldEqual, "users/user/myenv-0.8.1")
			So(statuses[0].Requested, ShouldEqual, mb.requested[0])
			// Requested: ,
			// BuildStart: ,
			// BuildDone: ,

			resp, err = server.Client().Post(server.URL+"/environments/build", "application/json", //nolint:noctx
				strings.NewReader(`
{
	"name": "users/user/myotherenv",
	"version": "1",
	"model": {
		"description": "help text",
		"packages": [{"name": "xxhash", "version": "0.8.1"}]
	}
}
`))
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			mb.requested = append(mb.requested, time.Now())

			resp, err = server.Client().Get(server.URL + "/environments/status") //nolint:noctx
			So(err, ShouldBeNil)
			So(resp.StatusCode, ShouldEqual, http.StatusOK)
			statuses = []build.Status{}
			err = json.NewDecoder(resp.Body).Decode(&statuses)
			So(err, ShouldBeNil)
			So(len(statuses), ShouldEqual, 2)
			So(statuses[0].Name, ShouldEqual, "users/user/myenv-0.8.1")
			So(statuses[1].Name, ShouldEqual, "users/user/myotherenv-1")
			So(statuses[1].Requested, ShouldEqual, mb.requested[1])
		})
	})
}

func TestServerReal(t *testing.T) {
	Convey("WIth a real builder", t, func() {
		ms3 := &internal.MockS3{}
		mwr := &internal.MockWR{Ch: make(chan struct{})}
		mc := &core.MockCore{Files: make(map[string]string)}
		msc := httptest.NewServer(mc)

		gm, _ := gitmock.New()

		gmhttp := httptest.NewServer(gm)

		var conf config.Config
		conf.S3.BinaryCache = "s3://spack"
		conf.S3.BuildBase = "some_path"
		conf.CustomSpackRepo = gmhttp.URL
		conf.CoreURL = msc.URL
		conf.Spack.BuildImage = "spack/ubuntu-jammy:v0.20.1"
		conf.Spack.FinalImage = "ubuntu:22.04"
		conf.Spack.ProcessorTarget = "x86_64_v4"

		builder, err := build.New(&conf, ms3, mwr)
		So(err, ShouldBeNil)

		handler := New(builder)
		server := httptest.NewServer(handler)
		So(server, ShouldNotBeNil)
	})
}
