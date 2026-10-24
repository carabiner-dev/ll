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
		Use:   "grant <role> <object_type:object_id> <subject_type:subject_id[#relation]>",
		Short: "Grant a role to a subject on an object",
		Long: `Grant a role to a subject on an object.

This assigns the named role to the subject on the specified object,
creating all the relation tuples defined by the role. A subject set (the
members of a group) is written with its relation: group:eng#member.

Examples:
  llctl role grant admin folder:my-folder user:alice
  llctl role grant viewer document:doc1 user:bob
  llctl role grant viewer document:doc1 group:eng#member`,
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
			subjectType, subjectID, subjectRelation, err := parseSubject(args[2])
			if err != nil {
				return err
			}

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close() //nolint:errcheck

			assignment, err := c.GrantRole(
				cmd.Context(),
				roleName,
				objectParts[0], objectParts[1],
				subjectType, subjectID, subjectRelation,
			)
			if err != nil {
				return err
			}

			fmt.Printf("granted role %q to %s on %s:%s (assignment: %s)\n",
				roleName,
				subjectString(assignment.GetSubjectType(), assignment.GetSubjectId(), assignment.GetSubjectRelation()),
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
		Use:   "revoke <role> <object_type:object_id> <subject_type:subject_id[#relation]>",
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
			subjectType, subjectID, subjectRelation, err := parseSubject(args[2])
			if err != nil {
				return err
			}

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close() //nolint:errcheck

			err = c.RevokeRole(
				cmd.Context(),
				roleName,
				objectParts[0], objectParts[1],
				subjectType, subjectID, subjectRelation,
			)
			if err != nil {
				return err
			}

			fmt.Printf("revoked role %q from %s on %s:%s\n",
				roleName,
				subjectString(subjectType, subjectID, subjectRelation),
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
	ObjectType      string
	ObjectID        string
	SubjectType     string
	SubjectID       string
	SubjectRelation string
	Role            string
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
	cmd.Flags().StringVar(&o.SubjectRelation, "subject-relation", "", "Filter by subject relation (a subject set's)")
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
			defer c.Close() //nolint:errcheck

			assignments, err := c.ListRoleAssignments(
				cmd.Context(),
				opts.ObjectType, opts.ObjectID,
				opts.SubjectType, opts.SubjectID, opts.SubjectRelation,
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
				fmt.Printf("%s: %s on %s:%s -> %s (created: %s)\n",
					a.GetRole(),
					a.GetId(),
					a.GetObjectType(), a.GetObjectId(),
					subjectString(a.GetSubjectType(), a.GetSubjectId(), a.GetSubjectRelation()),
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

This shows every role configured on the server with the grants it makes:
the relation each creates, on the object the role is granted on unless the
grant names another object as a template.`,
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
				if r.GetDescription() != "" {
					fmt.Printf("  - %s: %s\n", r.GetName(), r.GetDescription())
				} else {
					fmt.Printf("  - %s\n", r.GetName())
				}
				if len(r.GetAppliesTo()) > 0 {
					fmt.Printf("      on %s\n", strings.Join(r.GetAppliesTo(), ", "))
				} else {
					fmt.Println("      on any type that defines its relations")
				}
				for _, g := range r.GetGrants() {
					if g.GetObject() != "" {
						fmt.Printf("      %s on %s\n", g.GetRelation(), g.GetObject())
					} else {
						fmt.Printf("      %s\n", g.GetRelation())
					}
				}
			}
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

// parseSubject reads a subject as type:id, with #relation for a subject
// set (group:eng#member).
func parseSubject(arg string) (subjectType, subjectID, subjectRelation string, err error) {
	parts := strings.SplitN(arg, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("invalid subject format %q, expected type:id[#relation]", arg)
	}
	subjectID, subjectRelation, _ = strings.Cut(parts[1], "#")
	if subjectID == "" {
		return "", "", "", fmt.Errorf("invalid subject format %q, expected type:id[#relation]", arg)
	}
	return parts[0], subjectID, subjectRelation, nil
}

// subjectString writes a subject back as type:id[#relation].
func subjectString(subjectType, subjectID, subjectRelation string) string {
	if subjectRelation != "" {
		return subjectType + ":" + subjectID + "#" + subjectRelation
	}
	return subjectType + ":" + subjectID
}
