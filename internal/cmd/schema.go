// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/carabiner-dev/command"
	"github.com/spf13/cobra"
)

var _ command.OptionsSet = (*SchemaOptions)(nil)

// SchemaOptions contains the options for schema commands.
type SchemaOptions struct {
	ServerOptions
}

var defaultSchemaOptions = SchemaOptions{
	ServerOptions: defaultServerOptions,
}

func (o *SchemaOptions) Validate() error {
	return errors.Join(
		o.ServerOptions.Validate(),
	)
}

func (o *SchemaOptions) AddFlags(cmd *cobra.Command) {
	o.ServerOptions.AddFlags(cmd)
}

func (o *SchemaOptions) Config() *command.OptionsSetConfig {
	return nil
}

// AddSchema adds the schema command and its subcommands to the parent command.
func AddSchema(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Manage authorization schema",
		Long:  `Commands for managing named authorization schema sets.`,
	}

	addSchemaApply(cmd)
	addSchemaGet(cmd)
	addSchemaDelete(cmd)
	parent.AddCommand(cmd)
}

// AddApply adds a hidden "apply" command as an alias for "schema apply".
func AddApply(parent *cobra.Command) {
	opts := defaultSchemaOptions

	cmd := &cobra.Command{
		Use:    "apply <file.yaml>",
		Short:  "Apply authorization schema from YAML file (alias for 'schema apply')",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			if err := c.WriteSchema(cmd.Context(), string(data)); err != nil {
				return err
			}

			fmt.Println("schema applied successfully")
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

// AddGet adds a hidden "get" command as an alias for "schema get".
func AddGet(parent *cobra.Command) {
	opts := defaultSchemaOptions
	var all bool

	cmd := &cobra.Command{
		Use:    "get [sets|set <name>]",
		Short:  "Get authorization schema (alias for 'schema get')",
		Hidden: true,
		Args:   cobra.MaximumNArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No args and no --all flag: show help
			if len(args) == 0 && !all {
				return cmd.Help()
			}

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			// --all flag: get all schemas concatenated
			if all {
				yamlStr, err := c.ReadSchema(cmd.Context())
				if err != nil {
					return err
				}
				fmt.Print(yamlStr)
				return nil
			}

			// "sets" lists all schema set names
			if len(args) > 0 && args[0] == "sets" {
				names, err := c.ListSchemaSets(cmd.Context())
				if err != nil {
					return err
				}
				for _, name := range names {
					fmt.Println(name)
				}
				if len(names) == 0 {
					fmt.Println("no schema sets found")
				}
				return nil
			}

			// "set <name>" gets a specific schema set
			if len(args) >= 2 && args[0] == "set" {
				yamlStr, err := c.ReadSchemaSet(cmd.Context(), args[1])
				if err != nil {
					return err
				}
				fmt.Print(yamlStr)
				return nil
			}

			return fmt.Errorf("invalid arguments: use 'get --all', 'get sets', or 'get set <name>'")
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Get all schema sets concatenated")
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

func addSchemaApply(parent *cobra.Command) {
	opts := defaultSchemaOptions

	cmd := &cobra.Command{
		Use:   "apply <file.yaml>",
		Short: "Apply authorization schema from YAML file",
		Long: `Apply an authorization schema to the server from a YAML file.

The schema file should be in Kubernetes-style manifest format:

  apiVersion: v1
  kind: LamplightTypeDefinitionSet
  metadata:
    name: my-schema
  spec:
    - name: user
    - name: document
      relations:
        - name: viewer
          types:
            - type: user

Examples:
  llctl schema apply schema.yaml
  llctl schema apply --server=localhost:9090 schema.yaml`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			if err := c.WriteSchema(cmd.Context(), string(data)); err != nil {
				return err
			}

			fmt.Println("schema applied successfully")
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

func addSchemaGet(parent *cobra.Command) {
	opts := defaultSchemaOptions
	var all bool

	cmd := &cobra.Command{
		Use:   "get [sets|set <name>]",
		Short: "Get authorization schema",
		Long: `Get and display authorization schema from the server.

Usage:
  llctl schema get --all        # get all schemas concatenated
  llctl schema get sets         # list all schema set names
  llctl schema get set <name>   # get a specific schema set

Examples:
  llctl schema get --all
  llctl schema get sets
  llctl schema get set user-and-folder`,
		Args: cobra.MaximumNArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No args and no --all flag: show help
			if len(args) == 0 && !all {
				return cmd.Help()
			}

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			// --all flag: get all schemas concatenated
			if all {
				yamlStr, err := c.ReadSchema(cmd.Context())
				if err != nil {
					return err
				}
				fmt.Print(yamlStr)
				return nil
			}

			// "sets" lists all schema set names
			if len(args) > 0 && args[0] == "sets" {
				names, err := c.ListSchemaSets(cmd.Context())
				if err != nil {
					return err
				}
				for _, name := range names {
					fmt.Println(name)
				}
				if len(names) == 0 {
					fmt.Println("no schema sets found")
				}
				return nil
			}

			// "set <name>" gets a specific schema set
			if len(args) >= 2 && args[0] == "set" {
				yamlStr, err := c.ReadSchemaSet(cmd.Context(), args[1])
				if err != nil {
					return err
				}
				fmt.Print(yamlStr)
				return nil
			}

			return fmt.Errorf("invalid arguments: use 'get --all', 'get sets', or 'get set <name>'")
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Get all schema sets concatenated")
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

func addSchemaDelete(parent *cobra.Command) {
	opts := defaultSchemaOptions

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a schema set",
		Long: `Delete a named schema set from the server.

Examples:
  llctl schema delete user-and-folder
  llctl schema delete --server=localhost:9090 my-schema`,
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			if err := c.DeleteSchemaSet(cmd.Context(), args[0]); err != nil {
				return err
			}

			fmt.Printf("schema set %q deleted successfully\n", args[0])
			return nil
		},
	}
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}
