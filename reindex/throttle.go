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
	"sync"
	"time"
)

// Throttler lets you run a function limited to certain times.
type Throttler struct {
	op             func()
	period         time.Duration
	requestedStop  chan struct{}
	requriesSignal bool
	started        bool
	blocked        bool
	stopped        bool
	runner         *opRunner
	sync.Mutex
}

// NewThrottle returns a Throttler. Setting requiresSignal to true means that
// you must call Signal() for op to be run in the next period.
func NewThrottle(op func(), period time.Duration, requiresSignal bool) *Throttler {
	return &Throttler{
		op:             op,
		period:         period,
		requriesSignal: requiresSignal,
		blocked:        requiresSignal,
	}
}

// Start starts running our op every one of our periods, but not if the
// function is still running, and optionally only if a signal has been
// received.
func (t *Throttler) Start() {
	t.Lock()
	defer t.Unlock()

	if t.started {
		t.stop()
	}

	t.requestedStop = make(chan struct{})
	t.started = true
	t.stopped = false

	go t.runEveryPeriod()
}

func (t *Throttler) runEveryPeriod() {
	t.runner = &opRunner{
		op: t.op,
	}

	ticker := time.NewTicker(t.period)
	defer ticker.Stop()

	t.runIfSignalled()

	for {
		select {
		case <-ticker.C:
			t.runIfSignalled()

		case <-t.requestedStop:
			return
		}
	}
}

func (t *Throttler) runIfSignalled() {
	t.Lock()
	defer t.Unlock()

	if t.blocked {
		return
	}

	ran := t.runner.run()
	t.blocked = t.requriesSignal && ran
}

type opRunner struct {
	op func()
	sync.Mutex
	running bool
}

// run runs our op unless we're already running. Returns true if we ran this
// time.
func (o *opRunner) run() bool {
	o.Lock()
	if o.running {
		o.Unlock()

		return false
	}

	o.running = true
	o.Unlock()

	go func() {
		o.op()

		o.Lock()
		o.running = false
		o.Unlock()
	}()

	return true
}

// Signal allows our op to be run when the next period starts, if we were
// configured with requiresSignal.
func (t *Throttler) Signal() {
	t.Lock()
	defer t.Unlock()

	t.blocked = false
}

// Stop stops us doing any further runs of our op, but doesn't stop any ongoing
// run.
func (t *Throttler) Stop() {
	t.Lock()
	defer t.Unlock()

	t.stop()
}

func (t *Throttler) stop() {
	if t.stopped {
		return
	}

	close(t.requestedStop)
	t.stopped = true
	t.started = false
}
