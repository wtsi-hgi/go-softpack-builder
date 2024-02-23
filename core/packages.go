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

package core

import "github.com/wtsi-hgi/go-softpack-builder/internal"

const (
	ErrNoPackages    = internal.Error("packages required")
	ErrNoPackageName = internal.Error("package names required")
)

// Package describes the name and optional version of a spack package.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Validate returns an error if Name isn't set.
func (p *Package) Validate() error {
	if p.Name == "" {
		return ErrNoPackageName
	}

	return nil
}

type Packages []Package

// Validate returns an error if p is zero length, or any of its Packages are
// invalid.
func (p Packages) Validate() error {
	if len(p) == 0 {
		return ErrNoPackages
	}

	for _, pkg := range p {
		err := pkg.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}
