package cmd

import (
	"fmt"

	"github.com/RichieSams/sitegen/pkg"

	"github.com/spf13/cobra"
)

type buildOpts struct {
	ConfigPath string
}

func createBuildCmd() *cobra.Command {
	opts := buildOpts{}

	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Build the static site",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if opts.ConfigPath == "" {
				return fmt.Errorf("--config is a required argument")
			}

			cmd.SilenceUsage = true
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			err := pkg.BuildSite(opts.ConfigPath)
			if err != nil {
				return err
			}

			return nil
		},
	}

	buildCmd.Flags().StringVarP(&opts.ConfigPath, "config", "c", opts.ConfigPath, "Path to the configuration yaml file")
	return buildCmd
}
