package cmd

import (
	"log"

	"github.com/RichieSams/sitegen/pkg"

	"github.com/spf13/cobra"
)

var serveOpts struct {
	ConfigPath string
	Port       int
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Build and serve the static site, re-building on any file changes",
	PreRun: func(cmd *cobra.Command, args []string) {
		if serveOpts.ConfigPath == "" {
			log.Fatal("--config is a required argument")
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := pkg.BuildSite(serveOpts.ConfigPath)
		if err != nil {
			log.Fatalf("%v", err)
		}

		err = pkg.Serve(serveOpts.ConfigPath, serveOpts.Port)
		if err != nil {
			log.Fatalf("%v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVarP(&serveOpts.ConfigPath, "config", "c", serveOpts.ConfigPath, "Path to the configuration yaml file")
	serveCmd.Flags().IntVarP(&serveOpts.Port, "port", "p", 3456, "The port to serve on")
}
