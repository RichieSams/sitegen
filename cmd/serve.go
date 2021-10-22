package cmd

import (
	"log"

	"github.com/RichieSams/sitegen/pkg"

	"github.com/spf13/cobra"
)

type serveOpts struct {
	ConfigPath string
	Port       int
}

func createServeCmd() *cobra.Command {
	opts := serveOpts{
		Port: 3456,
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Build and serve the static site, re-building on any file changes",
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

			err = pkg.Serve(opts.ConfigPath, opts.Port)
			if err != nil {
				log.Fatalf("%v", err)
			}
		},
	}

	serveCmd.Flags().StringVarP(&opts.ConfigPath, "config", "c", opts.ConfigPath, "Path to the configuration yaml file")
	serveCmd.Flags().IntVarP(&opts.Port, "port", "p", opts.Port, "The port to serve on")

	return serveCmd
}
