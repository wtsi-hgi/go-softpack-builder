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
	"sync"

	"github.com/wtsi-hgi/go-softpack-builder/config"
)

// Builder can tell us when a build takes place.
type Builder interface {
	SetPostBuildCallback(func())
}

type Reindexer struct {
	conf *config.Config
	sync.Mutex
}

func New(conf *config.Config) *Reindexer {
	return &Reindexer{conf: conf}
}

// Reindex runs `spack buildcache update-index` on the configured S3
// BinaryCache.
//
// If a reindex is currently running, queues the reindex until after the current
// run.
//
// Logs when the reindex actually starts running, and when it ends, or if it
// fails.
func (r *Reindexer) Reindex() {
	r.Lock()
	defer r.Unlock()

	slog.Info("reindex started")

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

	slog.Info("reindex finished")
}
