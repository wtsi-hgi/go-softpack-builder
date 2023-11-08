package cmd

import (
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

		if err := remove.Remove(conf, s, args[1], args[2]); err != nil {
			die(err.Error())
		}
	},
}

func init() {
	RootCmd.AddCommand(removeCmd)
}
