package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wtsi-hgi/go-softpack-builder/config"
	"github.com/wtsi-hgi/go-softpack-builder/remove"
	"github.com/wtsi-hgi/go-softpack-builder/s3"
)

const numArgs = 2

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove an environment",
	Long: `Remove an environment.

Remove an existing environment; it's entry in the Core artefacts repo, the
module files and the singularity image and symlinks.

Usage: gsb remove softpack/env/path version
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			die("environment path required")
		}

		if len(args) < numArgs {
			die("environment version required")
		}

		if len(args) != numArgs {
			die("unexpected arguments")
		}

		conf, err := config.GetConfig(configPath)
		if err != nil {
			die("could not load config: %s", err)
		}

		s, err := s3.New(conf.S3.BuildBase)
		if err != nil {
			die(err.Error())
		}

		envPath := cleanEnvPath(args[0])

		if envPath != args[0] {
			die("invalid environment path")
		}

		cliPrint(
			"Will now remove environment %s-%s from artefacts repo and modules.\n"+
				"Are you sure you sure you wish to proceed? [yN]: ",
			envPath,
			args[1],
		)

		var resp string

		fmt.Scan(&resp)

		if resp != "y" {
			return
		}

		if err := remove.Remove(conf, s, envPath, args[1]); err != nil {
			die(err.Error())
		}
	},
}

func init() {
	RootCmd.AddCommand(removeCmd)
}

// cleanEnvPath strips out any attempts to manipulate the envPath in order to
// get GSB to remove incorrect files.
func cleanEnvPath(envPath string) string {
	return filepath.Clean(string(filepath.Separator) + envPath)[1:]
}
