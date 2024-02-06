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

package reindex

import (
	"os/exec"
	"time"

	"github.com/wtsi-hgi/go-softpack-builder/config"
)

// Builder can tell us when a build takes place.
type Builder interface {
	SetPostBuildCallback(func())
}

// Scheduler periodically updates the Spack buildcache index.
type Scheduler struct {
	throt   *Throttler
	conf    *config.Config
	builder Builder
	quit    chan struct{}
}

// NewScheduler returns a scheduler that can update the Spack buildcache index
// periodically.
func NewScheduler(conf *config.Config, builder Builder) *Scheduler {
	reindex := func() {
		cmd := exec.Command("spack", "buildcache", "update-index", "--", conf.S3.BinaryCache) //nolint:gosec
		// TODO: log the error
		_ = cmd.Run()
	}

	s := &Scheduler{
		throt:   NewThrottle(reindex, hoursToDuration(conf.Spack.ReindexHours), true),
		conf:    conf,
		builder: builder,
		quit:    make(chan struct{}),
	}

	builder.SetPostBuildCallback(s.Signal)

	return s
}

func hoursToDuration(hours float64) time.Duration {
	return time.Duration(hours*millisecondsInHour) * time.Millisecond
}

// Start starts the scheduler, calling `spack buildcache update-index`
// immediately.
func (s *Scheduler) Start() {
	s.throt.Start()
}

// Signal signals that a build has happened and a reindex is now needed.
func (s *Scheduler) Signal() {
	s.throt.Signal()
}

// Stop prevents future invocations of `spack buildcache update-index` (but
// does not interrupt an existing process).
func (s *Scheduler) Stop() {
	s.throt.Stop()
	close(s.quit)
}
