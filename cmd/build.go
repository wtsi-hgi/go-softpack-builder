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
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/core"
	"golang.org/x/sys/unix"
)

// Options for this sub-command.
var buildPath, buildVersion, buildDescription, buildPackagesPath, buildURL string

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build an environment",
	Long: `Build an environment.

Allows manual builds without a softpack client.`,
	Run: func(cmd *cobra.Command, args []string) {
		conf, err := config.GetConfig(configPath)
		if err != nil {
			die("could not load config: %s", err)
		}

		c, err := core.New(conf)
		if err != nil {
			die("failed to load core config: %s", err)
		}

		path := readInput("Enter environment path: ", buildPath)
		desc := readInput("Enter environment description (single line): ", buildDescription)
		pkgs := getPackageList(buildPackagesPath)
		err = c.Create(path, desc, pkgs)
		if err != nil {
			die("failed to create environment (but it might be queued for later): %s", err)
		}

		info("environment build successfully scheduled")
	},
}

func init() {
	RootCmd.AddCommand(buildCmd)

	buildCmd.Flags().StringVarP(&buildPath, "path", "p", "", "environment path")
	buildCmd.Flags().StringVarP(&buildDescription, "description", "d", "", "environment description")
	buildCmd.Flags().StringVarP(&buildPackagesPath, "packages", "k", "-", "file with list of packages, one per line")
	buildCmd.Flags().StringVarP(&buildURL, "url", "u", os.Getenv("GSB_URL"), "URL to running GSB server")
}

func readInput(prompt, given string) string {
	if given != "" {
		return given
	}

	fmt.Print(prompt)

	var v string

	fmt.Scanln(&v)

	return v
}

const pkgNameParts = 2

func getPackageList(path string) core.Packages {
	pkgsBytes := readPackageInput(path)

	pkgList := strings.Split(string(pkgsBytes), "\n")

	pkgs := make(core.Packages, len(pkgList))

	for n, pkgStr := range pkgList {
		parts := strings.SplitN(strings.TrimSpace(pkgStr), "@", pkgNameParts)

		pkgs[n].Name = parts[0]

		if len(parts) == pkgNameParts {
			pkgs[n].Version = parts[1]
		}
	}

	return pkgs
}

func readPackageInput(path string) []byte {
	var pkgsFile *os.File

	if path == "-" {
		printIfTTY("Enter Packages (Ctrl+d to end): ")

		pkgsFile = os.Stdin
	} else {
		pkgsFile = openOrDie(path)

		defer pkgsFile.Close()
	}

	pkgsBytes, err := io.ReadAll(pkgsFile)
	if err != nil {
		die("failed to read packages: %s", err)
	}

	return bytes.TrimSpace(pkgsBytes)
}

func printIfTTY(msg string) {
	if _, err := unix.IoctlGetWinsize(int(os.Stdin.Fd()), unix.TIOCGWINSZ); err != nil {
		return
	}

	cliPrint(msg)
}

func openOrDie(path string) *os.File {
	f, err := os.Open(path)
	if err != nil {
		die("failed to open file: %s", err)
	}

	return f
}
