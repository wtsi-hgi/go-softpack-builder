/*******************************************************************************
 * Copyright (c) 2023, 2024 Genome Research Ltd.
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

	"github.com/wtsi-hgi/go-softpack-builder/build"
)

const (
	endpointEnvs       = "/environments"
	endpointEnvsBuild  = endpointEnvs + "/build"
	endpointEnvsStatus = endpointEnvs + "/status"
)

type Error string

func (e Error) Error() string {
	return string(e)
}

// Builder interface describes anything that can Build() a singularity image
// given a build.Definition.
type Builder interface {
	Build(*build.Definition) error
	Status() []build.Status
}

// A Request object contains all of the information required to build an
// environment.
type Request struct {
	Name    string
	Version string `json:"omitempty"`
	Model   struct {
		Description string
		Packages    []build.Package
	}
}

// New takes a Builder that will be sent a Definition when the returned Handler
// receives request JSON POSTed to /environments/build, and uses the Builder to
// get status information for builds when it receives a GET request to
// /environments/status.
func New(b Builder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case endpointEnvsBuild:
			handleEnvBuild(b, w, r)
		case endpointEnvsStatus:
			handleEnvStatus(b, w)
		default:
			http.Error(w, fmt.Sprintf("go-softpack-builder: no such endpoint: %s", r.URL.Path), http.StatusNotFound)
		}
	})
}

func handleEnvBuild(b Builder, w http.ResponseWriter, r *http.Request) {
	req := new(Request)

	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		http.Error(w, fmt.Sprintf("error parsing request: %s", err), http.StatusBadRequest)

		return
	}

	def := new(build.Definition)
	def.EnvironmentPath, def.EnvironmentName = path.Split(req.Name)
	def.EnvironmentVersion = req.Version
	def.Description = req.Model.Description
	def.Packages = req.Model.Packages

	if err := def.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("error validating request: %s", err), http.StatusBadRequest)
	}

	if err := b.Build(def); err != nil {
		http.Error(w, fmt.Sprintf("error starting build: %s", err), http.StatusInternalServerError)
	}
}

func handleEnvStatus(b Builder, w http.ResponseWriter) {
	err := json.NewEncoder(w).Encode(b.Status())
	if err != nil {
		http.Error(w, fmt.Sprintf("error serialising status: %s", err), http.StatusInternalServerError)
	}
}
