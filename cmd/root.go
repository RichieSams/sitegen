package cmd

import (
	"github.com/spf13/cobra"
)

// Creates the root Command
// This can be executed to run the cli
func CreateRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sitegen",
		Short: "sitegen is a flexible static website generator",
	}

	rootCmd.AddCommand(createBuildCmd())
	rootCmd.AddCommand(createServeCmd())

	return rootCmd
}
