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
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/config"
)

func TestCore(t *testing.T) {
	Convey("Given a path, description and packages", t, func() {
		path := "users/foo/env"
		desc := "a desc"
		pkgs := Packages{
			{
				Name:    "pckA",
				Version: "1",
			},
			{

				Name:    "pckB",
				Version: "2",
			},
		}

		Convey("You can create a gqlQuery", func() {
			gq := newCreateMutation(path, desc, pkgs)
			So(gq, ShouldResemble, &gqlQuery{
				Variables: gqlVariables{
					Name:        "env",
					Path:        "users/foo",
					Description: desc,
					Packages:    pkgs,
				},
				Query: createMutationQuery,
			})
		})

		conf, err := config.GetConfig("")
		if err != nil || conf.CoreURL == "" || conf.ListenURL == "" {
			_, err = New(conf)
			So(err, ShouldNotBeNil)

			SkipConvey("Skipping further tests; set CoreURL and ListenURL in config file", func() {})

			return
		}

		core, err := New(conf)
		So(err, ShouldBeNil)

		var buildRequests int

		buildHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			buildRequests++
		})

		fakeBuildServer := httptest.NewUnstartedServer(buildHandler)
		l, err := net.Listen("tcp", conf.ListenURL)
		So(err, ShouldBeNil)
		fakeBuildServer.Listener.Close()
		fakeBuildServer.Listener = l
		fakeBuildServer.Start()
		defer fakeBuildServer.Close()

		Convey("You can create an environment", func() {
			So(buildRequests, ShouldEqual, 0)
			err = core.Create(path, desc, pkgs)
			So(err, ShouldBeNil)
			So(buildRequests, ShouldEqual, 1)

			repoPath := path + "-1"

			defer core.Delete(repoPath) //nolint:errcheck

			Convey("Then remove it", func() {
				err = core.Delete(repoPath)
				So(err, ShouldBeNil)

				err = core.Delete(repoPath)
				So(err, ShouldNotBeNil)
			})

			Convey("Then retrigger its creation", func() {
				err = core.ResendPendingBuilds()
				So(err, ShouldBeNil)
				So(buildRequests, ShouldBeGreaterThan, 1)
			})
		})

		Convey("You can't create an environment with empty path", func() {
			err := core.Create("", desc, pkgs)
			So(err, ShouldNotBeNil)
		})
	})
}
