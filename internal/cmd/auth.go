// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	deadropcmd "github.com/carabiner-dev/deadrop/pkg/cmd"
	"github.com/spf13/cobra"
)

// AddAuth adds the auth command and its subcommands to the parent.
func AddAuth(parent *cobra.Command) {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with Carabiner",
		Long: `Manage authentication with Carabiner.

The auth commands help you log in and out of the Carabiner ecosystem.
Login obtains a Carabiner identity token that is used by llctl to obtain
service-specific tokens for Lamplight.`,
	}

	// Expose deadrop commands directly
	deadropcmd.AddLogin(authCmd)
	deadropcmd.AddLogout(authCmd)
	deadropcmd.AddToken(authCmd)

	// Add whoami to show identity and grants from the server
	addAuthWhoAmI(authCmd)

	parent.AddCommand(authCmd)
}

func addAuthWhoAmI(parent *cobra.Command) {
	opts := defaultServerOptions

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Display the current user's identity and IAM grants",
		Long: `Display the authenticated user's identity as seen by the server,
along with their IAM grants on Lamplight operations.

This command is useful for:
  - Verifying your authentication is working correctly
  - Checking what permissions you have on Lamplight operations
  - Debugging authorization issues

Examples:
  # Check your identity and grants
  llctl auth whoami

  # With explicit server and token
  llctl auth whoami --server=localhost:8080 --token=/path/to/token.jwt`,
		Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close() //nolint:errcheck

			resp, err := c.WhoAmI(cmd.Context())
			if err != nil {
				return fmt.Errorf("calling WhoAmI: %w", err)
			}

			// Display identity
			fmt.Println("Identity:")
			if resp.GetSubject() != "" {
				fmt.Printf("  Subject: %s\n", resp.GetSubject())
			} else {
				fmt.Println("  Subject: (not authenticated)")
			}
			fmt.Printf("  Authentication enabled: %v\n", resp.GetAuthenticated())

			// Display grants
			fmt.Println("\nIAM Grants:")
			if len(resp.GetGrants()) == 0 {
				if resp.GetSubject() == "" {
					fmt.Println("  (not authenticated - no grants)")
				} else {
					fmt.Println("  (no grants found)")
				}
			} else {
				for _, grant := range resp.GetGrants() {
					fmt.Printf("  - %s#%s\n", grant.GetObject(), grant.GetRelation())
					if grant.GetDescription() != "" {
						fmt.Printf("    %s\n", grant.GetDescription())
					}
				}
			}

			return nil
		},
	}

	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}
