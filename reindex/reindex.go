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

const reindexHoursDividerForCheckingLastBuildTime = 2

// BuildTimeTracker can tell when a build last took place.
type BuildTimeTracker interface {
	LastBuiltTime() time.Time
}

// Scheduler periodically updates the Spack buildcache index.
type Scheduler struct {
	throt *Throttler
	conf  *config.Config
	btt   BuildTimeTracker
	quit  chan struct{}
}

// NewScheduler returns a scheduler that can update the Spack buildcache index
// periodically.
func NewScheduler(conf *config.Config, btt BuildTimeTracker) *Scheduler {
	reindex := func() {
		cmd := exec.Command("spack", "buildcache", "update-index", "--", conf.S3.BinaryCache) //nolint:gosec
		// TODO: log the error
		_ = cmd.Run()
	}

	s := &Scheduler{
		throt: NewThrottle(reindex, hoursToDuration(conf.Spack.ReindexHours), true),
		conf:  conf,
		btt:   btt,
		quit:  make(chan struct{}),
	}

	go s.watchForNewBuilds()

	return s
}

func hoursToDuration(hours float64) time.Duration {
	return time.Duration(hours*millisecondsInHour) * time.Millisecond
}

func (s *Scheduler) watchForNewBuilds() {
	ticker := time.NewTicker(hoursToDuration(s.conf.Spack.ReindexHours / reindexHoursDividerForCheckingLastBuildTime))
	defer ticker.Stop()

	var lbt time.Time

	for {
		select {
		case <-ticker.C:
			newLBT := s.btt.LastBuiltTime()
			if newLBT != lbt {
				lbt = newLBT

				s.throt.Signal()
			}
		case <-s.quit:
			return
		}
	}
}

// Start starts the scheduler, calling `spack buildcache update-index`
// immediately.
func (s *Scheduler) Start() {
	s.throt.Start()
}

// Stop prevents future invocations of `spack buildcache update-index` (but
// does not interrupt an existing process).
func (s *Scheduler) Stop() {
	s.throt.Stop()
	close(s.quit)
}
