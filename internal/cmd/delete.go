// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"

	"github.com/carabiner-dev/command"
	"github.com/spf13/cobra"

	llv1 "github.com/carabiner-dev/ll/api/carabiner/ll/v1"
)

var _ command.OptionsSet = (*DeleteOptions)(nil)

// DeleteOptions contains the options for the delete command.
type DeleteOptions struct {
	ServerOptions
	ObjectType string
	ObjectID   string
	Relation   string
}

var defaultDeleteOptions = DeleteOptions{
	ServerOptions: defaultServerOptions,
}

func (o *DeleteOptions) Validate() error {
	return errors.Join(
		o.ServerOptions.Validate(),
	)
}

func (o *DeleteOptions) AddFlags(cmd *cobra.Command) {
	o.ServerOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.ObjectType, "object-type", "", "Filter by object type")
	cmd.Flags().StringVar(&o.ObjectID, "object-id", "", "Filter by object ID")
	cmd.Flags().StringVar(&o.Relation, "relation", "", "Filter by relation")
}

func (o *DeleteOptions) Config() *command.OptionsSetConfig {
	return nil
}

// AddDelete adds the delete command to the parent command.
func AddDelete(parent *cobra.Command) {
	opts := defaultDeleteOptions

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete relation tuples",
		Long: `Delete relation tuples matching the specified filters.

Examples:
  llctl delete --object-type=document --object-id=doc1
  llctl delete --object-type=document --object-id=doc1 --relation=viewer`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			filter := &llv1.RelationTupleFilter{
				ObjectType: opts.ObjectType,
				ObjectId:   opts.ObjectID,
				Relation:   opts.Relation,
			}

			if err := c.Delete(cmd.Context(), filter); err != nil {
				return err
			}

			fmt.Println("deleted matching tuples")
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}
