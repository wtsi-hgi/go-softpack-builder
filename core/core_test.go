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
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/wtsi-hgi/go-softpack-builder/build"
	"github.com/wtsi-hgi/go-softpack-builder/internal"
)

func TestCore(t *testing.T) {
	Convey("Given a path, description and packages", t, func() {
		path := "users/foo/env"
		desc := "a desc"
		pkgs := build.Packages{
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

		Convey("You can create an environment", func() {
			conf, err := internal.GetConfig("")
			if err != nil || conf.CoreURL == "" {
				SkipConvey("Skipping test, set CoreURL in config file", func() {})

				return
			}

			core := New(conf)

			resp, err := core.Create(path, desc, pkgs)
			So(err, ShouldBeNil)
			So(string(resp), ShouldContainSubstring, "Successfully scheduled environment creation")
		})
	})
}
