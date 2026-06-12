// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/carabiner-dev/command"
	"github.com/spf13/cobra"

	"github.com/carabiner-dev/ll"
)

var _ command.OptionsSet = (*CheckOptions)(nil)

// CheckOptions contains the options for the check command.
type CheckOptions struct {
	ServerOptions
}

var defaultCheckOptions = CheckOptions{
	ServerOptions: defaultServerOptions,
}

func (o *CheckOptions) Validate() error {
	return errors.Join(
		o.ServerOptions.Validate(),
	)
}

func (o *CheckOptions) AddFlags(cmd *cobra.Command) {
	o.ServerOptions.AddFlags(cmd)
}

func (o *CheckOptions) Config() *command.OptionsSetConfig {
	return nil
}

// AddCheck adds the check command to the parent command.
func AddCheck(parent *cobra.Command) {
	opts := defaultCheckOptions

	cmd := &cobra.Command{
		Use:   "check <tuple>",
		Short: "Check if a subject has a permission on an object",
		Long: `Check permission using a tuple string.

Tuple format: object_type:object_id#relation@subject_type:subject_id[#subject_relation]

Examples:
  llctl check "document:doc1#can_view@user:alice"
  llctl check "folder:root#view@user:bob"`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := ll.ParseTuple(args[0])
			if err != nil {
				return fmt.Errorf("invalid tuple: %w", err)
			}

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			allowed, err := c.Check(cmd.Context(), t)
			if err != nil {
				return err
			}

			if allowed {
				fmt.Println("ALLOWED")
			} else {
				fmt.Println("DENIED")
				os.Exit(1)
			}
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}
