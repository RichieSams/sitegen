package main

import (
	"log"

	"github.com/RichieSams/sitegen/cmd"
)

func main() {
	rootCmd := cmd.CreateRootCmd()
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
