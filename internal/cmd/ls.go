// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"
	"os"

	"github.com/carabiner-dev/command"
	"github.com/carabiner-dev/ll"
	llv1 "github.com/carabiner-dev/ll/api/carabiner/ll/v1"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var _ command.OptionsSet = (*LsOptions)(nil)

// LsOptions contains the options for the ls command.
type LsOptions struct {
	ServerOptions
	Decode bool
	System bool
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
	cmd.Flags().BoolVar(&o.System, "system", false, "Include internal _lamplight system types and permissions")
}

func (o *LsOptions) Config() *command.OptionsSetConfig {
	return nil
}

// AddLs adds the ls command and its subcommands to the parent command.
func AddLs(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List grants, objects, or permissions",
		Long:  `Commands for listing grants, objects, and permissions.`,
	}

	addLsGrants(cmd)
	addLsObjects(cmd)
	addLsPermissions(cmd)
	parent.AddCommand(cmd)
}

// addLsGrants adds the grants subcommand (formerly the main ls command).
func addLsGrants(parent *cobra.Command) {
	opts := defaultLsOptions

	cmd := &cobra.Command{
		Use:   "grants <subject_type:subject_id#permission> <object_type>",
		Short: "List objects of a type that a subject has a permission on",
		Long: `List all objects of a given type that a subject has a specific permission on.

The subject is specified as subject_type:subject_id#permission, and the object type
is the type of objects to search.

Examples:
  llctl ls grants "user:alice#can_view" document
  llctl ls grants "user:bob#can_edit" folder`,
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

			c, err := opts.NewClient(cmd.Context())
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

// addLsObjects adds the objects subcommand.
func addLsObjects(parent *cobra.Command) {
	opts := defaultLsOptions

	cmd := &cobra.Command{
		Use:   "objects [type]",
		Short: "List object types or objects of a specific type",
		Long: `List registered object types, or list all known objects of a specific type.

Without arguments, lists all object types defined in the schema.
With a type argument, lists all objects of that type that have tuples.

By default, internal _lamplight system types are hidden. Use --system to show them.

Examples:
  llctl ls objects              # list all object types
  llctl ls objects --system     # include _lamplight system types
  llctl ls objects folder       # list all folders with tuples`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			// No args: list object types from schema
			if len(args) == 0 {
				schemaYAML, err := c.ReadSchema(cmd.Context())
				if err != nil {
					return fmt.Errorf("reading schema: %w", err)
				}

				types, err := parseObjectTypes(schemaYAML)
				if err != nil {
					return fmt.Errorf("parsing schema: %w", err)
				}

				// Filter out system types unless --system flag is set
				var filtered []typeInfo
				for _, t := range types {
					if opts.System || !strings.HasPrefix(t.Name, "_lamplight.") {
						filtered = append(filtered, t)
					}
				}

				if len(filtered) == 0 {
					fmt.Println("no object types found")
					return nil
				}

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "TYPE\tDESCRIPTION")
				for _, t := range filtered {
					fmt.Fprintf(w, "%s\t%s\n", t.Name, t.Description)
				}
				w.Flush()
				return nil
			}

			// With type arg: list objects of that type
			objectType := args[0]
			filter := &llv1.RelationTupleFilter{
				ObjectType: objectType,
			}

			tuples, err := c.Read(cmd.Context(), filter)
			if err != nil {
				return err
			}

			// Extract unique object IDs
			seen := make(map[string]bool)
			var objectIDs []string
			for _, t := range tuples {
				if !seen[t.ObjectId] {
					seen[t.ObjectId] = true
					objectIDs = append(objectIDs, t.ObjectId)
				}
			}

			for _, id := range objectIDs {
				if opts.Decode {
					fmt.Println(ll.DecodeID(id))
				} else {
					fmt.Println(id)
				}
			}

			if len(objectIDs) == 0 {
				fmt.Printf("no %s objects found\n", objectType)
			}
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

// addLsPermissions adds the permissions subcommand.
func addLsPermissions(parent *cobra.Command) {
	opts := defaultLsOptions

	cmd := &cobra.Command{
		Use:   "permissions [type|type:id]",
		Short: "List permissions from schema or grants on an object",
		Long: `List permissions defined in the schema or permission grants on a specific object.

Without arguments, lists all permissions in a table with their object types.
With a type argument, lists permissions for that specific type.
With a type:id argument, lists permission grants on that specific object instance.

By default, internal _lamplight system types are hidden. Use --system to show them.

Examples:
  llctl ls permissions                  # list all permissions by type
  llctl ls permissions --system         # include _lamplight system permissions
  llctl ls permissions folder           # list permissions for folder type
  llctl ls permissions folder:home      # list grants on folder:home`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			// No args: list all permissions by type
			if len(args) == 0 {
				schemaYAML, err := c.ReadSchema(cmd.Context())
				if err != nil {
					return fmt.Errorf("reading schema: %w", err)
				}

				perms, err := parseAllPermissions(schemaYAML)
				if err != nil {
					return fmt.Errorf("parsing schema: %w", err)
				}

				// Filter out system types unless --system flag is set
				var filtered []typePermission
				for _, p := range perms {
					if opts.System || !strings.HasPrefix(p.ObjectType, "_lamplight.") {
						filtered = append(filtered, p)
					}
				}

				if len(filtered) == 0 {
					fmt.Println("no permissions found")
					return nil
				}

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "TYPE\tPERMISSION")
				for _, p := range filtered {
					fmt.Fprintf(w, "%s\t%s\n", p.ObjectType, p.Permission)
				}
				w.Flush()
				return nil
			}

			arg := args[0]

			// Check if it's type:id (has a colon)
			colonIdx := strings.Index(arg, ":")
			if colonIdx > 0 {
				// type:id - list permission grants on that object
				objectType := arg[:colonIdx]
				objectID := arg[colonIdx+1:]

				filter := &llv1.RelationTupleFilter{
					ObjectType: objectType,
					ObjectId:   objectID,
				}

				tuples, err := c.Read(cmd.Context(), filter)
				if err != nil {
					return err
				}

				if len(tuples) == 0 {
					fmt.Printf("no grants found on %s\n", arg)
					return nil
				}

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "RELATION\tSUBJECT")
				for _, t := range tuples {
					subject := fmt.Sprintf("%s:%s", t.SubjectType, t.SubjectId)
					if opts.Decode {
						subject = fmt.Sprintf("%s:%s", t.SubjectType, ll.DecodeID(t.SubjectId))
					}
					if t.SubjectRelation != "" {
						subject += "#" + t.SubjectRelation
					}
					fmt.Fprintf(w, "%s\t%s\n", t.Relation, subject)
				}
				w.Flush()
				return nil
			}

			// Just type - list permissions for that type from schema
			objectType := arg
			schemaYAML, err := c.ReadSchema(cmd.Context())
			if err != nil {
				return fmt.Errorf("reading schema: %w", err)
			}

			perms, err := parseTypePermissions(schemaYAML, objectType)
			if err != nil {
				return fmt.Errorf("parsing schema: %w", err)
			}

			for _, p := range perms {
				fmt.Println(p)
			}

			if len(perms) == 0 {
				fmt.Printf("no permissions found for type %q\n", objectType)
			}
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

// Schema parsing helpers

// schemaDoc represents a parsed schema document.
type schemaDoc struct {
	Spec []schemaType `yaml:"spec"`
}

// schemaType represents a type definition in the schema.
type schemaType struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description"`
	Permissions []schemaPermission `yaml:"permissions"`
}

// schemaPermission represents a permission definition.
type schemaPermission struct {
	Name string `yaml:"name"`
}

// typePermission pairs a type with a permission name.
type typePermission struct {
	ObjectType string
	Permission string
}

// typeInfo holds type name and description.
type typeInfo struct {
	Name        string
	Description string
}

// parseObjectTypes extracts all type names and descriptions from the schema YAML.
func parseObjectTypes(schemaYAML string) ([]typeInfo, error) {
	var types []typeInfo

	decoder := yaml.NewDecoder(strings.NewReader(schemaYAML))
	for {
		var doc schemaDoc
		if err := decoder.Decode(&doc); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}

		for _, t := range doc.Spec {
			if t.Name != "" {
				types = append(types, typeInfo{
					Name:        t.Name,
					Description: t.Description,
				})
			}
		}
	}

	return types, nil
}

// parseAllPermissions extracts all permissions with their types from the schema.
func parseAllPermissions(schemaYAML string) ([]typePermission, error) {
	var perms []typePermission

	decoder := yaml.NewDecoder(strings.NewReader(schemaYAML))
	for {
		var doc schemaDoc
		if err := decoder.Decode(&doc); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}

		for _, t := range doc.Spec {
			for _, p := range t.Permissions {
				if p.Name != "" {
					perms = append(perms, typePermission{
						ObjectType: t.Name,
						Permission: p.Name,
					})
				}
			}
		}
	}

	return perms, nil
}

// parseTypePermissions extracts permissions for a specific type from the schema.
func parseTypePermissions(schemaYAML string, objectType string) ([]string, error) {
	var perms []string

	decoder := yaml.NewDecoder(strings.NewReader(schemaYAML))
	for {
		var doc schemaDoc
		if err := decoder.Decode(&doc); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}

		for _, t := range doc.Spec {
			if t.Name == objectType {
				for _, p := range t.Permissions {
					if p.Name != "" {
						perms = append(perms, p.Name)
					}
				}
			}
		}
	}

	return perms, nil
}
