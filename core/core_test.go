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
	"fmt"
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
		if err != nil || conf.CoreURL == "" {
			fmt.Printf("\nerr: %s\n", err)
			_, err = New(conf)
			So(err, ShouldNotBeNil)

			SkipConvey("Skipping further tests; set CoreURL in config file", func() {})

			return
		}

		core, err := New(conf)
		So(err, ShouldBeNil)

		buildHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
		fakeBuildServer := httptest.NewUnstartedServer(buildHandler)
		fakeBuildServer.Config.Addr = conf.ListenURL
		fakeBuildServer.Start()
		defer fakeBuildServer.Close()

		Convey("You can create an environment", func() {
			// err = core.Delete(path + "-1")
			// So(err, ShouldBeNil)
			err = core.Create(path, desc, pkgs)
			So(err, ShouldBeNil)

			Convey("Then remove it", func() {
				err = core.Delete(path + "-1")
				So(err, ShouldBeNil)

				err = core.Delete(path + "-1")
				So(err, ShouldNotBeNil)
			})

			Convey("Then retrigger its creation", func() {
				err = core.ResendPendingBuilds()
				So(err, ShouldBeNil)

				// if conf.ListenURL == "" {
				// 	SkipConvey("Skipping resend tests; set ListenURL in config file")

				// 	return
				// }

				// var body string

				// coreHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// 	content, err := io.ReadAll(r.Body)
				// 	if err != nil {
				// 		http.Error(w, fmt.Sprintf("read failed: %s", err), http.StatusInternalServerError)

				// 		return
				// 	}

				// 	body = string(content)
				// })

				// testServer := httptest.NewUnstartedServer(coreHandler)
				// testServer.Config.Addr = conf.ListenURL
				// testServer.Start()
				// defer testServer.Close()

				// So(body, ShouldEqual, "foo")
			})
		})

		Convey("You can't create an environment with empty path", func() {
			err := core.Create("", desc, pkgs)
			So(err, ShouldNotBeNil)
		})
	})
}
