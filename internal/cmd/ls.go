// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/carabiner-dev/command"
	"github.com/carabiner-dev/ll"
	"github.com/spf13/cobra"
)

var _ command.OptionsSet = (*LsOptions)(nil)

// LsOptions contains the options for the ls command.
type LsOptions struct {
	ServerOptions
	Decode bool
}

var defaultLsOptions = LsOptions{
	ServerOptions: defaultServerOptions,
	Decode:        true,
}

func (o *LsOptions) Validate() error {
	return errors.Join(
		o.ServerOptions.Validate(),
	)
}

func (o *LsOptions) AddFlags(cmd *cobra.Command) {
	o.ServerOptions.AddFlags(cmd)
	cmd.Flags().BoolVar(&o.Decode, "decode", true, "Decode IDs containing special characters (show human-readable form)")
}

func (o *LsOptions) Config() *command.OptionsSetConfig {
	return nil
}

// AddLs adds the ls command to the parent command.
func AddLs(parent *cobra.Command) {
	opts := defaultLsOptions

	cmd := &cobra.Command{
		Use:   "ls <subject_type:subject_id#permission> <object_type>",
		Short: "List objects of a type that a subject has a permission on",
		Long: `List all objects of a given type that a subject has a specific permission on.

The subject is specified as subject_type:subject_id#permission, and the object type
is the type of objects to search.

Examples:
  llctl ls "user:alice#can_view" document
  llctl ls "user:bob#can_edit" folder`,
		Args: cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse subject_type:subject_id#permission
			subjectPerm := args[0]
			objectType := args[1]

			hashIdx := strings.Index(subjectPerm, "#")
			if hashIdx < 0 {
				return fmt.Errorf("invalid format: expected subject_type:subject_id#permission, got %q", subjectPerm)
			}

			subjectRef := subjectPerm[:hashIdx]
			permission := subjectPerm[hashIdx+1:]

			colonIdx := strings.Index(subjectRef, ":")
			if colonIdx < 0 {
				return fmt.Errorf("invalid format: expected subject_type:subject_id, got %q", subjectRef)
			}

			subjectType := subjectRef[:colonIdx]
			subjectID := subjectRef[colonIdx+1:]

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

			objects, err := c.ListObjects(cmd.Context(), subjectType, subjectID, permission, objectType)
			if err != nil {
				return err
			}

			for _, id := range objects {
				if opts.Decode {
					fmt.Println(ll.DecodeID(id))
				} else {
					fmt.Println(id)
				}
			}

			if len(objects) == 0 {
				fmt.Println("no objects found")
			}
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}
