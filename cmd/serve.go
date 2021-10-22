package cmd

import (
	"fmt"

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

			err = pkg.Serve(opts.ConfigPath, opts.Port)
			if err != nil {
				return err
			}

			return nil
		},
	}

	serveCmd.Flags().StringVarP(&opts.ConfigPath, "config", "c", opts.ConfigPath, "Path to the configuration yaml file")
	serveCmd.Flags().IntVarP(&opts.Port, "port", "p", opts.Port, "The port to serve on")

	return serveCmd
}
