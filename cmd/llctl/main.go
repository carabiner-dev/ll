// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/carabiner-dev/ll/internal/cmd"
	"github.com/spf13/cobra"
)

var version = "dev" // Set via ldflags during build

func main() {
	rootCmd := &cobra.Command{
		Use:   "llctl",
		Short: "llctl - CLI client for the Lamplight permissions engine",
		Long: `llctl is a command-line client for interacting with the Lamplight
permissions engine, a Google Zanzibar-style authorization system.

It provides commands for:
  - Checking permissions
  - Writing and reading relation tuples
  - Managing authorization schemas
  - Listing objects a subject has access to`,
	}

	// Add commands
	cmd.AddCheck(rootCmd)
	cmd.AddWrite(rootCmd)
	cmd.AddRead(rootCmd)
	cmd.AddDelete(rootCmd)
	cmd.AddSchema(rootCmd)
	cmd.AddLs(rootCmd)
	addVersion(rootCmd)

	// Hidden aliases
	cmd.AddApply(rootCmd)
	cmd.AddGet(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func addVersion(parent *cobra.Command) {
	parent.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("llctl version %s\n", version)
		},
	})
}
