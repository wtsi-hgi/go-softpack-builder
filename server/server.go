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
	"net"
	"net/http"
	"path"
	"time"

	"github.com/wtsi-hgi/go-softpack-builder/build"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/core"
	"gopkg.in/tylerb/graceful.v1"
)

const (
	endpointEnvs            = "/environments"
	endpointEnvsBuild       = endpointEnvs + "/build"
	endpointEnvsStatus      = endpointEnvs + "/status"
	stopTimeout             = 10 * time.Second
	readHeaderTimeout       = 20 * time.Second
	waitUntilStartedTimeout = 30 * time.Second
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
	Version string `json:"version,omitempty"`
	Model   struct {
		Description string
		Packages    []core.Package
	}
}

type Server struct {
	b         Builder
	srv       *graceful.Server
	c         *core.Core
	startedCh chan struct{}
}

// New takes a Builder that will be sent a Definition when the returned Handler
// receives request JSON POSTed to /environments/build, and uses the Builder to
// get status information for builds when it receives a GET request to
// /environments/status. It uses the config to get your core URL, and if set
// will trigger the core service to resend pending builds to us after Start().
func New(b Builder, c *config.Config) *Server {
	s := &Server{
		b: b,
	}

	cor, err := core.New(c)
	if err == nil {
		s.c = cor
		s.startedCh = make(chan struct{})
	}

	return s
}

// Start starts a server using the given listener (you can get one from
// NewListener()). Blocks until Stop() is called, or a kill signal is received.
//
// You should always defer Stop(), regardless of this returning an error.
//
// If we had been configured with core details, core will be asked to resend its
// queued environments.
func (s *Server) Start(l net.Listener) error {
	s.srv = &graceful.Server{
		Timeout: stopTimeout,

		Server: &http.Server{
			Handler:           s.endpointsHandler(),
			ReadHeaderTimeout: readHeaderTimeout,
		},
	}

	errCh := make(chan error, 1)

	go func() {
		errCh <- s.srv.Serve(l)
	}()

	err := s.resendPendingBuildsIfCoreConfigured()
	if err != nil {
		return err
	}

	return <-errCh
}

func (s *Server) endpointsHandler() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case endpointEnvsBuild:
			handleEnvBuild(s.b, w, r)
		case endpointEnvsStatus:
			handleEnvStatus(s.b, w)
		default:
			http.Error(w, fmt.Sprintf("go-softpack-builder: no such endpoint: %s", r.URL.Path), http.StatusNotFound)
		}
	})
}

func (s *Server) resendPendingBuildsIfCoreConfigured() error {
	if s.c == nil {
		return nil
	}

	err := s.c.ResendPendingBuilds()
	close(s.startedCh)

	return err
}

// WaitUntilStarted blocks until Start() brings up a listener and the core
// service talk to us. Will only work if we were configured with core details
// and the core is working. There is a timeout; will return false if the timeout
// expires, or core details weren't configured.
func (s *Server) WaitUntilStarted() bool {
	if s.startedCh == nil {
		return false
	}

	timeout := time.NewTimer(waitUntilStartedTimeout)

	select {
	case <-s.startedCh:
		return true
	case <-timeout.C:
		return false
	}
}

// NewListener returns a listener that listens on the given URL. If blank,
// listens on a random localhost port. You can use l.Addr().String() to get the
// address.
func NewListener(listenURL string) (net.Listener, error) {
	if listenURL == "" {
		listenURL = "127.0.0.1:0"
	}

	return net.Listen("tcp", listenURL)
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

func (s *Server) Stop() {
	s.srv.Stop(stopTimeout)
}
