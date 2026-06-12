// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/carabiner-dev/command"
	"github.com/spf13/cobra"
)

var _ command.OptionsSet = (*RoleOptions)(nil)

// RoleOptions contains the options for role commands.
type RoleOptions struct {
	ServerOptions
}

var defaultRoleOptions = RoleOptions{
	ServerOptions: defaultServerOptions,
}

func (o *RoleOptions) Validate() error {
	return errors.Join(
		o.ServerOptions.Validate(),
	)
}

func (o *RoleOptions) AddFlags(cmd *cobra.Command) {
	o.ServerOptions.AddFlags(cmd)
}

func (o *RoleOptions) Config() *command.OptionsSetConfig {
	return nil
}

// AddRole adds the role command group to the parent command.
func AddRole(parent *cobra.Command) {
	roleCmd := &cobra.Command{
		Use:   "role",
		Short: "Manage role assignments",
		Long: `Manage role assignments in the Lamplight permissions engine.

Roles are predefined sets of relation tuples that can be applied atomically
to a subject on an object. When a role is granted, all the tuples defined
in the role are created. When revoked, all those tuples are removed.

Tuples created via roles are "locked" - they cannot be deleted individually
and can only be removed by revoking the role.`,
	}

	addRoleGrant(roleCmd)
	addRoleRevoke(roleCmd)
	addRoleList(roleCmd)
	addRoleListDefinitions(roleCmd)

	parent.AddCommand(roleCmd)
}

func addRoleGrant(parent *cobra.Command) {
	opts := defaultRoleOptions

	cmd := &cobra.Command{
		Use:   "grant <role> <object_type:object_id> <subject_type:subject_id>",
		Short: "Grant a role to a subject on an object",
		Long: `Grant a role to a subject on an object.

This assigns the named role to the subject on the specified object,
creating all the relation tuples defined by the role.

Examples:
  llctl role grant admin folder:my-folder user:alice
  llctl role grant viewer document:doc1 user:bob`,
		Args: cobra.ExactArgs(3),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			roleName := args[0]
			objectParts := strings.SplitN(args[1], ":", 2)
			if len(objectParts) != 2 {
				return fmt.Errorf("invalid object format %q, expected type:id", args[1])
			}
			subjectParts := strings.SplitN(args[2], ":", 2)
			if len(subjectParts) != 2 {
				return fmt.Errorf("invalid subject format %q, expected type:id", args[2])
			}

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			assignment, err := c.GrantRole(
				cmd.Context(),
				roleName,
				objectParts[0], objectParts[1],
				subjectParts[0], subjectParts[1],
			)
			if err != nil {
				return err
			}

			fmt.Printf("granted role %q to %s:%s on %s:%s (assignment: %s)\n",
				roleName,
				assignment.GetSubjectType(), assignment.GetSubjectId(),
				assignment.GetObjectType(), assignment.GetObjectId(),
				assignment.GetId(),
			)
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

func addRoleRevoke(parent *cobra.Command) {
	opts := defaultRoleOptions

	cmd := &cobra.Command{
		Use:   "revoke <role> <object_type:object_id> <subject_type:subject_id>",
		Short: "Revoke a role from a subject on an object",
		Long: `Revoke a role from a subject on an object.

This removes the role assignment and deletes all the relation tuples
that were created when the role was granted.

Examples:
  llctl role revoke admin folder:my-folder user:alice
  llctl role revoke viewer document:doc1 user:bob`,
		Args: cobra.ExactArgs(3),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			roleName := args[0]
			objectParts := strings.SplitN(args[1], ":", 2)
			if len(objectParts) != 2 {
				return fmt.Errorf("invalid object format %q, expected type:id", args[1])
			}
			subjectParts := strings.SplitN(args[2], ":", 2)
			if len(subjectParts) != 2 {
				return fmt.Errorf("invalid subject format %q, expected type:id", args[2])
			}

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			err = c.RevokeRole(
				cmd.Context(),
				roleName,
				objectParts[0], objectParts[1],
				subjectParts[0], subjectParts[1],
			)
			if err != nil {
				return err
			}

			fmt.Printf("revoked role %q from %s:%s on %s:%s\n",
				roleName,
				subjectParts[0], subjectParts[1],
				objectParts[0], objectParts[1],
			)
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

// RoleListOptions contains options for the role list command.
type RoleListOptions struct {
	ServerOptions
	ObjectType  string
	ObjectID    string
	SubjectType string
	SubjectID   string
	Role        string
}

var defaultRoleListOptions = RoleListOptions{
	ServerOptions: defaultServerOptions,
}

func (o *RoleListOptions) Validate() error {
	return errors.Join(
		o.ServerOptions.Validate(),
	)
}

func (o *RoleListOptions) AddFlags(cmd *cobra.Command) {
	o.ServerOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.ObjectType, "object-type", "", "Filter by object type")
	cmd.Flags().StringVar(&o.ObjectID, "object-id", "", "Filter by object ID")
	cmd.Flags().StringVar(&o.SubjectType, "subject-type", "", "Filter by subject type")
	cmd.Flags().StringVar(&o.SubjectID, "subject-id", "", "Filter by subject ID")
	cmd.Flags().StringVar(&o.Role, "role", "", "Filter by role name")
}

func (o *RoleListOptions) Config() *command.OptionsSetConfig {
	return nil
}

func addRoleList(parent *cobra.Command) {
	opts := defaultRoleListOptions

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List role assignments",
		Long: `List role assignments with optional filters.

Examples:
  llctl role list
  llctl role list --object-type=folder --object-id=my-folder
  llctl role list --subject-type=user --subject-id=alice
  llctl role list --role=admin`,
		Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			assignments, err := c.ListRoleAssignments(
				cmd.Context(),
				opts.ObjectType, opts.ObjectID,
				opts.SubjectType, opts.SubjectID,
				opts.Role,
			)
			if err != nil {
				return err
			}

			if len(assignments) == 0 {
				fmt.Println("no role assignments found")
				return nil
			}

			for _, a := range assignments {
				fmt.Printf("%s: %s on %s:%s -> %s:%s (created: %s)\n",
					a.GetRole(),
					a.GetId(),
					a.GetObjectType(), a.GetObjectId(),
					a.GetSubjectType(), a.GetSubjectId(),
					a.GetCreatedAt(),
				)
			}
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

func addRoleListDefinitions(parent *cobra.Command) {
	opts := defaultRoleOptions

	cmd := &cobra.Command{
		Use:     "definitions",
		Aliases: []string{"defs"},
		Short:   "List available role definitions",
		Long: `List all role definitions available on the server.

This shows the names of all roles that have been configured on the server.`,
		Args: cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			roles, err := c.ListRoles(cmd.Context())
			if err != nil {
				return err
			}

			if len(roles) == 0 {
				fmt.Println("no roles defined")
				return nil
			}

			fmt.Println("Available roles:")
			for _, r := range roles {
				fmt.Printf("  - %s\n", r)
			}
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}
