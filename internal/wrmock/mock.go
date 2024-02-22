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

package wrmock

import (
	"sync"
	"time"

	"github.com/wtsi-hgi/go-softpack-builder/wr"
)

// MockWR can be used to test a build.Builder without having real wr running.
type MockWR struct {
	Cmd                   string
	Fail                  bool
	PollForStatusInterval time.Duration
	JobDuration           time.Duration

	sync.RWMutex
	ReturnStatus wr.WRJobStatus
}

func NewMockWR(pollForStatusInterval, jobDuration time.Duration) *MockWR {
	return &MockWR{
		PollForStatusInterval: pollForStatusInterval,
		JobDuration:           jobDuration,
	}
}

// Add implements build.Runner interface.
func (m *MockWR) Add(cmd string) (string, error) { //nolint:unparam
	m.Cmd = cmd

	return "abc123", nil
}

// SetRunning should be used before waiting on Ch and if you need to test
// BuildStart time in Status.
func (m *MockWR) SetRunning() {
	m.Lock()
	defer m.Unlock()

	m.ReturnStatus = wr.WRJobStatusRunning
}

func (m *MockWR) SetComplete() {
	m.Lock()
	defer m.Unlock()

	m.ReturnStatus = wr.WRJobStatusComplete
}

// WaitForRunning implements build.Runner interface.
func (m *MockWR) WaitForRunning(string) error { //nolint:unparam
	for {
		m.RLock()
		rs := m.ReturnStatus
		m.RUnlock()

		if rs == wr.WRJobStatusRunning || rs == wr.WRJobStatusBuried || rs == wr.WRJobStatusComplete {
			return nil
		}

		<-time.After(m.PollForStatusInterval)
	}
}

// Wait implements build.Runner interface.
func (m *MockWR) Wait(string) (wr.WRJobStatus, error) { //nolint:unparam
	<-time.After(m.JobDuration)

	if m.Fail {
		return wr.WRJobStatusBuried, nil
	}

	return wr.WRJobStatusComplete, nil
}

// Status implements build.Runner interface.
func (m *MockWR) Status(string) (wr.WRJobStatus, error) { //nolint:unparam
	m.RLock()
	defer m.RUnlock()

	return m.ReturnStatus, nil
}

func (m *MockWR) Cleanup() error { //nolint:unparam
	m.Lock()
	defer m.Unlock()

	m.ReturnStatus = wr.WRJobStatusInvalid

	return nil
}
