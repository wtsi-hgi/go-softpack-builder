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
	"fmt"
	"net/http"
	"path"

	"github.com/wtsi-hgi/go-softpack-builder/spack"
)

type Builder interface {
	Build(*spack.Definition) error
}

type request struct {
	Name    string
	Version string
	Model   struct {
		Description string
		Packages    []spack.Package
	}
}

// New takes a Builder that will be sent a Definition when the returned Handler
// receives request JSON.
func New(b Builder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := new(request)

		if err := json.NewDecoder(r.Body).Decode(req); err != nil {
			http.Error(w, fmt.Sprintf("error parsing request: %s", err), http.StatusBadRequest)

			return
		}

		def := new(spack.Definition)
		def.EnvironmentPath, def.EnvironmentName = path.Split(req.Name)
		def.EnvironmentVersion = req.Version
		def.Description = req.Model.Description
		def.Packages = req.Model.Packages

		if err := b.Build(def); err != nil {
			http.Error(w, fmt.Sprintf("error starting build: %s", err), http.StatusInternalServerError)
		}
	})
}
