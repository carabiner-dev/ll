// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/carabiner-dev/ll"
	"github.com/spf13/cobra"
)

// AddPath adds the path command group to the parent command.
func AddPath(parent *cobra.Command) {
	pathCmd := &cobra.Command{
		Use:   "path",
		Short: "Manage hierarchical paths",
		Long: `Manage hierarchical paths in the Lamplight permissions engine.

Paths provide a convenient way to establish parent-child relationships
between objects. Instead of manually creating individual relation tuples
for each link in a hierarchy, you can specify the entire path and Lamplight
will create the necessary tuples.

Path format: type:id>type:id>type:id#relation`,
	}

	addPathWrite(pathCmd)

	parent.AddCommand(pathCmd)
}

func addPathWrite(parent *cobra.Command) {
	opts := defaultServerOptions

	cmd := &cobra.Command{
		Use:   "write <path>",
		Short: "Write a hierarchical path, creating all parent tuples",
		Long: `Write a hierarchical path, ensuring all parent tuples exist.

The path format is: type:id>type:id>type:id#relation

This creates tuples connecting each child to its parent using the specified relation.
For example, the path:

  folder:root>folder:projects>file:readme.md#parent

Creates these tuples:
  folder:projects#parent@folder:root
  file:readme.md#parent@folder:projects

The operation is idempotent - existing tuples are not duplicated.

Special characters in object IDs should be URL-encoded:
  > as %3E
  # as %23
  : as %3A (only in the ID portion)
  @ as %40

Examples:
  llctl path write "folder:root>folder:sub>file:doc.txt#parent"
  llctl path write "org:acme>team:backend>repo:api#owner"
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
