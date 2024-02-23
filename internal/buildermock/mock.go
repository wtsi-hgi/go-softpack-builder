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

package buildermock

import (
	"path/filepath"
	"time"

	"github.com/wtsi-hgi/go-softpack-builder/build"
)

// MockBuilder can be used to test a server.Server without having real builder.
type MockBuilder struct {
	Received  []*build.Definition
	Requested []time.Time
}

// Build adds the given def to our slice of Received.
func (m *MockBuilder) Build(def *build.Definition) error { //nolint:unparam
	m.Received = append(m.Received, def)

	return nil
}

// Status returns a status for everything sent to Build, assuming you pushed
// a corresponding timestamp to our Requested slice manually.
func (m *MockBuilder) Status() []build.Status {
	statuses := make([]build.Status, 0, len(m.Received))

	for i, def := range m.Received {
		if len(m.Requested) <= i {
			break
		}

		statuses = append(statuses, build.Status{
			Name:      filepath.Join(def.EnvironmentPath, def.EnvironmentName) + "-" + def.EnvironmentVersion,
			Requested: &m.Requested[i],
		})
	}

	return statuses
}
