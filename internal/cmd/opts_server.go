// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/carabiner-dev/command"
	"github.com/carabiner-dev/deadrop/pkg/client/credentials"
	"github.com/spf13/cobra"

	"github.com/carabiner-dev/ll"
)

var _ command.OptionsSet = (*ServerOptions)(nil)

// LamplightAudience is the audience claim for Lamplight service tokens.
const LamplightAudience = "lamplight"

var defaultServerOptions = ServerOptions{
	Server:      "localhost:8080",
	AuthOptions: *credentials.NewServerOptions(credentials.WithPrefix("auth"), credentials.WithAudience(LamplightAudience)),
}

// ServerOptions contains the common server connection options.
type ServerOptions struct {
	// Server is the Lamplight server address.
	Server string

	// AuthOptions provides authentication via deadrop credentials manager.
	AuthOptions credentials.ServerOptions

	// Token is an explicit authentication token (file path or token string).
	// If set, bypasses the credentials manager.
	Token string

	// UseREST uses REST API instead of gRPC.
	UseREST bool

	// Insecure disables TLS (for local development).
	Insecure bool
}

func (so *ServerOptions) Config() *command.OptionsSetConfig {
	return nil
}

func (so *ServerOptions) Validate() error {
	if so.Server == "" {
		return errors.New("server address not set")
	}
	return nil
}

func (so *ServerOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&so.Server, "server", defaultServerOptions.Server, "Lamplight server address")
	cmd.PersistentFlags().StringVar(&so.Token, "token", "", "Authentication token (file path or token string, bypasses credentials manager)")
	cmd.PersistentFlags().BoolVar(&so.UseREST, "rest", false, "Use REST API instead of gRPC")
	cmd.PersistentFlags().BoolVar(&so.Insecure, "insecure", false, "Disable TLS (for local development)")

	// Add auth flags from deadrop credentials options (--auth-server)
	so.AuthOptions.AddFlags(cmd)
}

// GetToken returns the authentication token.
// If Token is explicitly set and looks like a file path, reads from file.
// If Token is explicitly set as a string, returns it as-is.
// If Token is empty, uses the credentials manager to get a token.
func (so *ServerOptions) GetToken(ctx context.Context) (string, error) {
	// If explicit token is provided, use it (bypasses credentials manager)
	if so.Token != "" {
		// Check if it's a file path
		if _, err := os.Stat(so.Token); err == nil {
			data, err := os.ReadFile(so.Token)
			if err != nil {
				return "", err
			}
			return strings.TrimSpace(string(data)), nil
		}
		// Not a file, return as-is
		return so.Token, nil
	}

	// No explicit token - use credentials manager via deadrop options
	source, err := so.AuthOptions.TokenSource(ctx)
	if err != nil {
		return "", err
	}

	token, err := source.Token(ctx)
	if err != nil {
		return "", err
	}

	return token, nil
}

// NewClient creates a new Lamplight client based on the configured options.
// If UseREST is true, it creates a REST client; otherwise, it creates a gRPC client.
func (so *ServerOptions) NewClient(ctx context.Context) (ll.Client, error) {
	token, err := so.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	if so.UseREST {
		var opts []ll.RESTOption
		if token != "" {
			opts = append(opts, ll.WithRESTToken(token))
		}
		if so.Insecure {
			opts = append(opts, ll.WithRESTInsecure())
		}
		return ll.NewREST(so.Server, opts...)
	}

	var opts []ll.Option
	if token != "" {
		opts = append(opts, ll.WithToken(token))
	}
	if so.Insecure {
		opts = append(opts, ll.WithInsecure())
	}
	return ll.New(so.Server, opts...)
}
