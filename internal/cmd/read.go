// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"

	"github.com/carabiner-dev/command"
	"github.com/carabiner-dev/ll"
	llv1 "github.com/carabiner-dev/ll/api/carabiner/ll/v1"
	"github.com/spf13/cobra"
)

var _ command.OptionsSet = (*ReadOptions)(nil)

// ReadOptions contains the options for the read command.
type ReadOptions struct {
	ServerOptions
	ObjectType string
	ObjectID   string
	Relation   string
	Decode     bool
}

var defaultReadOptions = ReadOptions{
	ServerOptions: defaultServerOptions,
	Decode:        true,
}

func (o *ReadOptions) Validate() error {
	return errors.Join(
		o.ServerOptions.Validate(),
	)
}

func (o *ReadOptions) AddFlags(cmd *cobra.Command) {
	o.ServerOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.ObjectType, "object-type", "", "Filter by object type")
	cmd.Flags().StringVar(&o.ObjectID, "object-id", "", "Filter by object ID")
	cmd.Flags().StringVar(&o.Relation, "relation", "", "Filter by relation")
	cmd.Flags().BoolVar(&o.Decode, "decode", true, "Decode IDs containing special characters (show human-readable form)")
}

func (o *ReadOptions) Config() *command.OptionsSetConfig {
	return nil
}

// AddRead adds the read command to the parent command.
func AddRead(parent *cobra.Command) {
	opts := defaultReadOptions

	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read relation tuples",
		Long: `Read relation tuples from the server with optional filters.

Examples:
  llctl read
  llctl read --object-type=document
  llctl read --object-type=document --object-id=doc1
  llctl read --object-type=document --relation=viewer`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := opts.GetToken()
			if err != nil {
				return fmt.Errorf("reading token: %w", err)
			}

			var clientOpts []ll.Option
			if token != "" {
				clientOpts = append(clientOpts, ll.WithToken(token))
			}

			c, err := ll.New(opts.Server, clientOpts...)
			if err != nil {
				return err
			}
			defer c.Close()

			filter := &llv1.RelationTupleFilter{
				ObjectType: opts.ObjectType,
				ObjectId:   opts.ObjectID,
				Relation:   opts.Relation,
			}

			tuples, err := c.Read(cmd.Context(), filter)
			if err != nil {
				return err
			}

			for _, t := range tuples {
				if opts.Decode {
					fmt.Println(ll.FormatTupleDecoded(t))
				} else {
					fmt.Println(ll.FormatTuple(t))
				}
			}

			if len(tuples) == 0 {
				fmt.Println("no tuples found")
			}
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}
