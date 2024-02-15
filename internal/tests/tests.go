/*******************************************************************************
 * Copyright (c) 2023 Genome Research Ltd.
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

package tests

import (
	"embed"
	"strings"
	"sync"
)

//go:embed testdata
var TestData embed.FS

// ConcurrentStringBuilder should be used when testing slog logging:
//
//	var logWriter internal.ConcurrentStringBuilder
//	slog.SetDefault(slog.New(slog.NewTextHandler(&logWriter, nil)))
type ConcurrentStringBuilder struct {
	mu sync.RWMutex
	strings.Builder
}

// Write is for implementing io.Writer.
func (c *ConcurrentStringBuilder) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Builder.Write(p)
}

// WriteString is for implementing io.Writer.
func (c *ConcurrentStringBuilder) WriteString(str string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.Builder.WriteString(str)
}

// String returns the current logs messages as a string.
func (c *ConcurrentStringBuilder) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.Builder.String()
}
