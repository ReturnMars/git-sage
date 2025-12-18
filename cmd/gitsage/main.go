// Package main is the entry point for the GitSage CLI application.
// GitSage is an AI-powered command-line tool that automatically generates
// semantic Git commit messages based on staged changes.
package main

import (
	"fmt"
	"os"

	"github.com/gitsage/gitsage/internal/cmd"
)

// Version information - set via ldflags during build
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	rootCmd := cmd.NewRootCmd(version, commit, date)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
