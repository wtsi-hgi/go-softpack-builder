package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wtsi-hgi/go-softpack-builder/remove"
	"github.com/wtsi-hgi/go-softpack-builder/s3"
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove an environment",
	Long: `Remove an environment.

Remove an existing environment; it's entry in the Core artefacts repo, the
module files and the singularity image and symlinks.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			die("require a environment path")
		}

		if len(args) < 2 {
			die("require a environment version")
		}

		if len(args) != 2 {
			die("unexpected arguments")
		}

		conf := getConfig()

		s, err := s3.New(conf.S3.BuildBase)
		if err != nil {
			die(err.Error())
		}

		envPath := filepath.Clean(string(filepath.Separator) + args[0])[1:]

		if envPath != args[0] {
			die("invalid environment path")
		}

		fmt.Printf(
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
