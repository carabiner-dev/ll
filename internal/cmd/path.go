// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/carabiner-dev/ll"
	"github.com/spf13/cobra"
)

// AddEnsurePath adds the ensure-path command to the parent command.
func AddEnsurePath(parent *cobra.Command) {
	opts := defaultServerOptions

	cmd := &cobra.Command{
		Use:   "ensure-path <path>",
		Short: "Ensure all parent tuples exist for a hierarchical path",
		Long: `Ensure all parent tuples exist for a hierarchical path.

The path format is: type:id>type:id>type:id#relation

This creates tuples connecting each child to its parent using the specified relation.
For example, the path:

  folder:root>folder:projects>file:readme.md#parent

Creates these tuples:
  folder:projects#parent@folder:root
  file:readme.md#parent@folder:projects

Special characters in object IDs should be URL-encoded:
  > as %3E
  # as %23
  : as %3A (only in the ID portion)
  @ as %40

Examples:
  llctl ensure-path "folder:root>folder:sub>file:doc.txt#parent"
  llctl ensure-path "org:acme>team:backend>repo:api#owner"
`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			pathStr := args[0]

			// Validate the path first
			path, err := ll.ParsePath(pathStr)
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}

			c, err := opts.NewClient()
			if err != nil {
				return err
			}
			defer c.Close()

			if err := c.EnsurePath(cmd.Context(), pathStr); err != nil {
				return err
			}

			// Print what was created
			tuples := path.Tuples()
			fmt.Printf("Ensured %d parent tuple(s):\n", len(tuples))
			for _, t := range tuples {
				fmt.Printf("  %s:%s#%s@%s:%s\n",
					t.ObjectType, t.ObjectId, t.Relation,
					t.SubjectType, t.SubjectId)
			}

			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}
