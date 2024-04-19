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

package debounce

import (
	"sync"
)

// Debounce lets you run a function unless it is already running, queueing a run
// after any existing run.
type Debounce struct {
	op      func()
	running bool
	queued  bool
	wg      sync.WaitGroup
	sync.Mutex
}

// New returns a Debounce.
func New(op func()) *Debounce {
	return &Debounce{
		op: op,
	}
}

// Run starts running our op, but not if the function is still running. Instead,
// a single run will start after the existing one completes (regardless of how
// many Run()s were called during the existing run).
func (d *Debounce) Run() {
	d.Lock()
	defer d.Unlock()

	if d.running {
		d.queued = true

		return
	}

	d.running = true
	d.wg.Add(1)

	go func() {
		defer d.wg.Done()
		d.op()

		d.Lock()
		d.running = false

		if d.queued {
			d.queued = false

			d.Unlock()
			d.Run()

			return
		}

		d.Unlock()
	}()
}

// waits until all operations complete.
func (d *Debounce) Wait() {
	d.wg.Wait()
}
