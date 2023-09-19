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

package cmd

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/server"
	"github.com/wtsi-hgi/go-softpack-builder/spack"
)

func serverCmd(_ *cobra.Command, _ []string) {
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			die(err.Error())
		}

		configPath = filepath.Join(home, ".softpack", "builder", "gsb-config.yml")
	}

	f, err := os.Open(configPath)
	if err != nil {
		die(err.Error())
	}

	conf, err := config.Parse(f)
	f.Close()

	if err != nil {
		die(err.Error())
	}

	b, err := spack.New(conf)
	if err != nil {
		die(err.Error())
	}

	err = http.ListenAndServe(conf.ListenURL, server.New(b)) //nolint:gosec
	if err != nil {
		die(err.Error())
	}
}