// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"

	"github.com/carabiner-dev/command"
	"github.com/spf13/cobra"

	"github.com/carabiner-dev/ll"
	llv1 "github.com/carabiner-dev/ll/api/carabiner/ll/v1"
)

var _ command.OptionsSet = (*WriteOptions)(nil)

// WriteOptions contains the options for the write command.
type WriteOptions struct {
	ServerOptions
}

var defaultWriteOptions = WriteOptions{
	ServerOptions: defaultServerOptions,
}

func (o *WriteOptions) Validate() error {
	return errors.Join(
		o.ServerOptions.Validate(),
	)
}

func (o *WriteOptions) AddFlags(cmd *cobra.Command) {
	o.ServerOptions.AddFlags(cmd)
}

func (o *WriteOptions) Config() *command.OptionsSetConfig {
	return nil
}

// AddWrite adds the write command to the parent command.
func AddWrite(parent *cobra.Command) {
	opts := defaultWriteOptions

	cmd := &cobra.Command{
		Use:   "write <tuple> [<tuple>...]",
		Short: "Write relation tuples",
		Long: `Write one or more relation tuples to the server.

Tuple format: object_type:object_id#relation@subject_type:subject_id[#subject_relation]

Examples:
  llctl write "document:doc1#owner@user:alice"
  llctl write "document:doc1#viewer@user:bob" "document:doc1#viewer@user:charlie"
  llctl write "folder:root#viewer@group:eng#member"`,
		Args: cobra.MinimumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var tuples []*llv1.RelationTuple
			for _, arg := range args {
				t, err := ll.ParseTuple(arg)
				if err != nil {
					return fmt.Errorf("invalid tuple %q: %w", arg, err)
				}
				tuples = append(tuples, t)
			}

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close() //nolint:errcheck

			if err := c.Write(cmd.Context(), tuples, nil); err != nil {
				return err
			}

			fmt.Printf("wrote %d tuple(s)\n", len(tuples))
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}
