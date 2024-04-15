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
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/wtsi-hgi/go-softpack-builder/config"
)

// Builder can tell us when a build takes place.
type Builder interface {
	SetPostBuildCallback(func())
}

type Reindexer struct {
	conf *config.Config
}

func (r *Reindexer) Reindex() {
	cmd := exec.Command(r.conf.Spack.Path, "buildcache", "update-index", "--", r.conf.S3.BinaryCache) //nolint:gosec
	out, err := cmd.CombinedOutput()

	var outstr, errstr string

	if out != nil {
		outstr = string(out)
	}

	if err != nil {
		errstr = err.Error()
	}

	if err != nil || strings.Contains(outstr, "Error") {
		slog.Error("spack reindex failed", "err", errstr, "out", outstr)
	}
}

func New(conf *config.Config) *Reindexer {
	return &Reindexer{conf: conf}
}

// Scheduler periodically updates the Spack buildcache index.
type Scheduler struct {
	*Throttler
	conf    *config.Config
	builder Builder
}

// NewScheduler returns a scheduler that can update the Spack buildcache index
// periodically.
func NewScheduler(conf *config.Config, builder Builder) *Scheduler {
	reindex := func() {
		cmd := exec.Command(conf.Spack.Path, "buildcache", "update-index", "--", conf.S3.BinaryCache) //nolint:gosec
		out, err := cmd.CombinedOutput()

		var outstr, errstr string

		if out != nil {
			outstr = string(out)
		}

		if err != nil {
			errstr = err.Error()
		}

		if err != nil || strings.Contains(outstr, "Error") {
			slog.Error("spack reindex failed", "err", errstr, "out", outstr)
		}
	}

	s := &Scheduler{
		Throttler: NewThrottle(reindex, hoursToDuration(conf.Spack.ReindexHours), true),
		conf:      conf,
		builder:   builder,
	}

	builder.SetPostBuildCallback(s.Signal)

	return s
}

func hoursToDuration(hours float64) time.Duration {
	return time.Duration(hours * float64(time.Hour))
}
