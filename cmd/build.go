package cmd

import (
	"log"

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
		PreRun: func(cmd *cobra.Command, args []string) {
			if opts.ConfigPath == "" {
				log.Fatal("--config is a required argument")
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			err := pkg.BuildSite(opts.ConfigPath)
			if err != nil {
				log.Fatalf("%v", err)
			}
		},
	}

	buildCmd.Flags().StringVarP(&opts.ConfigPath, "config", "c", opts.ConfigPath, "Path to the configuration yaml file")
	return buildCmd
}
