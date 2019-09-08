package cmd

import (
	"log"

	"github.com/RichieSams/sitegen/pkg"

	"github.com/spf13/cobra"
)

var buildOpts struct {
	ConfigPath string
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the static site",
	PreRun: func(cmd *cobra.Command, args []string) {
		if buildOpts.ConfigPath == "" {
			log.Fatal("--config is a required argument")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := pkg.BuildSite(buildOpts.ConfigPath)
		if err != nil {
			log.Fatalf("%v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().StringVarP(&buildOpts.ConfigPath, "config", "c", buildOpts.ConfigPath, "Path to the configuration yaml file")
}
