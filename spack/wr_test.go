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

package spack

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestWR(t *testing.T) {
	Convey("You can generate a wr input", t, func() {
		// buildBase would come from our yml config, and envPath and Name
		// would come from the Definition from the core post to our Server.
		buildBase := "spack/builds/"
		envPath := "users/user"
		envName := "myenv"
		s3Path := buildBase + envPath + "/" + envName

		wrInput, err := GenerateWRAddInput(s3Path)
		So(err, ShouldBeNil)
		So(wrInput, ShouldEqual, `{"cmd": "echo doing build in spack/builds/users/user/myenv; sudo singularity build --force singularity.sif singularity.def", `+
			`"retries": 0, "rep_grp": "singularity_build-spack/builds/users/user/myenv", "limit_grps": ["s3cache"], `+
			`"mounts_json": [{"Targets": [{"Path":"spack/builds/users/user/myenv","Write":true,"Cache":true}]}]`)
	})
}
