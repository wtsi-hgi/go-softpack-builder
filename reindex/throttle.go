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

type Throttler struct {
	requestedStop chan struct{}
	stopped       bool
	sync.Mutex
}

func Throttle(op func(), period time.Duration) *Throttler {
	throt := &Throttler{
		requestedStop: make(chan struct{}),
	}

	go func() {
		runner := &opRunner{
			op: op,
		}

		ticker := time.NewTicker(period)
		defer ticker.Stop()

		runner.run()

		for {
			select {
			case <-ticker.C:
				runner.run()

			case <-throt.requestedStop:
				return
			}
		}
	}()

	return throt
}

type opRunner struct {
	op func()
	sync.Mutex
	running bool
}

// run runs our op unless we're already running.
func (o *opRunner) run() {
	o.Lock()
	if o.running {
		o.Unlock()
		return
	}
	o.running = true
	o.Unlock()

	go func() {
		o.op()

		o.Lock()
		o.running = false
		o.Unlock()
	}()
}

func (throt *Throttler) Stop() {
	throt.Lock()
	defer throt.Unlock()

	if throt.stopped {
		return
	}

	close(throt.requestedStop)
	throt.stopped = true
}
