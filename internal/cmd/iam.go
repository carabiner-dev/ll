// SPDX-FileCopyrightText: Copyright 2025 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/carabiner-dev/command"
	"github.com/carabiner-dev/ll"
	llv1 "github.com/carabiner-dev/ll/api/carabiner/ll/v1"
	"github.com/spf13/cobra"
)

var _ command.OptionsSet = (*IAMOptions)(nil)

// IAMOptions contains the options for IAM commands.
type IAMOptions struct {
	ServerOptions
}

var defaultIAMOptions = IAMOptions{
	ServerOptions: defaultServerOptions,
}

func (o *IAMOptions) Validate() error {
	return errors.Join(
		o.ServerOptions.Validate(),
	)
}

func (o *IAMOptions) AddFlags(cmd *cobra.Command) {
	o.ServerOptions.AddFlags(cmd)
}

func (o *IAMOptions) Config() *command.OptionsSetConfig {
	return nil
}

// AddIAM adds the iam command and its subcommands to the parent command.
func AddIAM(parent *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "iam",
		Short: "Manage fine-grained IAM permissions",
		Long: `Commands for managing fine-grained IAM permissions on objects.

IAM permissions control who can read or write tuples on specific objects.
Permissions can be granted on any object and will inherit down the object
hierarchy (via relations marked with lamplight.io/iam-parent annotation).

The _lamplight.tuple:global object is the root - grants there apply to all objects.`,
	}

	addIAMGrant(cmd)
	addIAMRevoke(cmd)
	addIAMCheck(cmd)
	parent.AddCommand(cmd)
}

// encodeObjectForIAM encodes a user object (type:id) as an IAM object ID.
func encodeObjectForIAM(objectType, objectID string) string {
	return ll.EncodeID(objectType + ":" + objectID)
}

// parseObjectRef parses an object reference like "folder:my-folder" into type and ID.
func parseObjectRef(ref string) (objectType, objectID string, err error) {
	idx := strings.Index(ref, ":")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid object reference %q: expected type:id format", ref)
	}
	return ref[:idx], ref[idx+1:], nil
}

// parseSubjectRef parses a subject reference like "user:alice" into type and ID.
func parseSubjectRef(ref string) (subjectType, subjectID string, err error) {
	idx := strings.Index(ref, ":")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid subject reference %q: expected type:id format", ref)
	}
	return ref[:idx], ref[idx+1:], nil
}

func addIAMGrant(parent *cobra.Command) {
	opts := defaultIAMOptions

	cmd := &cobra.Command{
		Use:   "grant <permission> <object> --to <subject>",
		Short: "Grant an IAM permission on an object",
		Long: `Grant an IAM permission (tuple.read or tuple.write) on an object to a subject.

The object can be:
  - "global" for global permissions (applies to all objects)
  - "type:id" for permissions on a specific object

The subject is typically a user identifier (e.g., user email).

Examples:
  # Grant alice global tuple write permission
  llctl iam grant tuple.write global --to alice@example.com

  # Grant bob tuple write permission on a specific folder
  llctl iam grant tuple.write folder:my-folder --to bob@example.com

  # Grant carol tuple read permission on a repository
  llctl iam grant tuple.read repository:my-repo --to carol@example.com`,
		Args: cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			permission := args[0]
			objectRef := args[1]

			// Validate permission
			if permission != "tuple.read" && permission != "tuple.write" {
				return fmt.Errorf("invalid permission %q: must be tuple.read or tuple.write", permission)
			}

			// Get the subject from --to flag
			subjectFlag, err := cmd.Flags().GetString("to")
			if err != nil || subjectFlag == "" {
				return fmt.Errorf("--to flag is required")
			}

			// Determine the relation based on permission
			var relation string
			switch permission {
			case "tuple.read":
				relation = "reader"
			case "tuple.write":
				relation = "writer"
			}

			// Determine the IAM object ID
			var iamObjectID string
			if objectRef == "global" {
				iamObjectID = "global"
			} else {
				objectType, objectID, err := parseObjectRef(objectRef)
				if err != nil {
					return err
				}
				iamObjectID = encodeObjectForIAM(objectType, objectID)
			}

			// Encode the subject
			encodedSubject := ll.EncodeID(subjectFlag)

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			// Create the IAM tuple
			tuple, err := ll.ParseTuple(fmt.Sprintf("_lamplight.tuple:%s#%s@_lamplight.user:%s",
				iamObjectID, relation, encodedSubject))
			if err != nil {
				return fmt.Errorf("creating IAM tuple: %w", err)
			}

			if err := c.Write(cmd.Context(), []*llv1.RelationTuple{tuple}, nil); err != nil {
				return fmt.Errorf("granting permission: %w", err)
			}

			if objectRef == "global" {
				fmt.Printf("Granted %s on all objects to %s\n", permission, subjectFlag)
			} else {
				fmt.Printf("Granted %s on %s to %s\n", permission, objectRef, subjectFlag)
			}
			return nil
		},
	}

	cmd.Flags().String("to", "", "Subject to grant permission to (required)")
	cmd.MarkFlagRequired("to") //nolint:errcheck
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

func addIAMRevoke(parent *cobra.Command) {
	opts := defaultIAMOptions

	cmd := &cobra.Command{
		Use:   "revoke <permission> <object> --from <subject>",
		Short: "Revoke an IAM permission on an object",
		Long: `Revoke an IAM permission (tuple.read or tuple.write) from a subject.

Examples:
  # Revoke alice's global tuple write permission
  llctl iam revoke tuple.write global --from alice@example.com

  # Revoke bob's tuple write permission on a specific folder
  llctl iam revoke tuple.write folder:my-folder --from bob@example.com`,
		Args: cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			permission := args[0]
			objectRef := args[1]

			// Validate permission
			if permission != "tuple.read" && permission != "tuple.write" {
				return fmt.Errorf("invalid permission %q: must be tuple.read or tuple.write", permission)
			}

			// Get the subject from --from flag
			subjectFlag, err := cmd.Flags().GetString("from")
			if err != nil || subjectFlag == "" {
				return fmt.Errorf("--from flag is required")
			}

			// Determine the relation based on permission
			var relation string
			switch permission {
			case "tuple.read":
				relation = "reader"
			case "tuple.write":
				relation = "writer"
			}

			// Determine the IAM object ID
			var iamObjectID string
			if objectRef == "global" {
				iamObjectID = "global"
			} else {
				objectType, objectID, err := parseObjectRef(objectRef)
				if err != nil {
					return err
				}
				iamObjectID = encodeObjectForIAM(objectType, objectID)
			}

			// Encode the subject
			encodedSubject := ll.EncodeID(subjectFlag)

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			// Create the IAM tuple to delete
			tuple, err := ll.ParseTuple(fmt.Sprintf("_lamplight.tuple:%s#%s@_lamplight.user:%s",
				iamObjectID, relation, encodedSubject))
			if err != nil {
				return fmt.Errorf("creating IAM tuple: %w", err)
			}

			if err := c.Write(cmd.Context(), nil, []*llv1.RelationTuple{tuple}); err != nil {
				return fmt.Errorf("revoking permission: %w", err)
			}

			if objectRef == "global" {
				fmt.Printf("Revoked %s on all objects from %s\n", permission, subjectFlag)
			} else {
				fmt.Printf("Revoked %s on %s from %s\n", permission, objectRef, subjectFlag)
			}
			return nil
		},
	}

	cmd.Flags().String("from", "", "Subject to revoke permission from (required)")
	cmd.MarkFlagRequired("from") //nolint:errcheck
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

func addIAMCheck(parent *cobra.Command) {
	opts := defaultIAMOptions

	cmd := &cobra.Command{
		Use:   "check <permission> <object> --subject <subject>",
		Short: "Check if a subject has an IAM permission on an object",
		Long: `Check if a subject has an IAM permission (tuple.read or tuple.write) on an object.

Examples:
  # Check if alice has global tuple write permission
  llctl iam check tuple.write global --subject alice@example.com

  # Check if bob has tuple write permission on a specific folder
  llctl iam check tuple.write folder:my-folder --subject bob@example.com`,
		Args: cobra.ExactArgs(2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			permission := args[0]
			objectRef := args[1]

			// Validate permission
			if permission != "tuple.read" && permission != "tuple.write" {
				return fmt.Errorf("invalid permission %q: must be tuple.read or tuple.write", permission)
			}

			// Get the subject from --subject flag
			subjectFlag, err := cmd.Flags().GetString("subject")
			if err != nil || subjectFlag == "" {
				return fmt.Errorf("--subject flag is required")
			}

			// Determine the IAM object ID
			var iamObjectID string
			if objectRef == "global" {
				iamObjectID = "global"
			} else {
				objectType, objectID, err := parseObjectRef(objectRef)
				if err != nil {
					return err
				}
				iamObjectID = encodeObjectForIAM(objectType, objectID)
			}

			// Encode the subject
			encodedSubject := ll.EncodeID(subjectFlag)

			c, err := opts.NewClient(cmd.Context())
			if err != nil {
				return err
			}
			defer c.Close()

			// Create the check tuple
			tuple, err := ll.ParseTuple(fmt.Sprintf("_lamplight.tuple:%s#%s@_lamplight.user:%s",
				iamObjectID, permission, encodedSubject))
			if err != nil {
				return fmt.Errorf("creating check tuple: %w", err)
			}

			allowed, err := c.Check(cmd.Context(), tuple)
			if err != nil {
				return fmt.Errorf("checking permission: %w", err)
			}

			if allowed {
				fmt.Printf("ALLOWED: %s has %s on %s\n", subjectFlag, permission, objectRef)
			} else {
				fmt.Printf("DENIED: %s does not have %s on %s\n", subjectFlag, permission, objectRef)
			}
			return nil
		},
	}

	cmd.Flags().String("subject", "", "Subject to check (required)")
	cmd.MarkFlagRequired("subject") //nolint:errcheck
	opts.AddFlags(cmd)
	parent.AddCommand(cmd)
}

