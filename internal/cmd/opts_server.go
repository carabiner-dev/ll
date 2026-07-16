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
	"github.com/carabiner-dev/deadrop/pkg/client/exchange"
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
	source, err := so.tokenSource()
	if err != nil {
		return "", err
	}

	token, err := source.Token(ctx)
	if err != nil {
		return "", err
	}

	return token, nil
}

// renewingIdentitySource loads the identity token cached for one specific auth
// server, renewing it when it has expired.
type renewingIdentitySource struct {
	serverURL string
}

func (r *renewingIdentitySource) Token(ctx context.Context) (string, error) {
	token, _, _, err := credentials.LoadIdentityWithRenewal(ctx, r.serverURL)
	if err != nil {
		return "", err
	}
	return token, nil
}

// tokenSource builds the service token source for --auth-server, exchanging the
// identity cached for that same server.
//
// This is deliberately not credentials.ServerOptions.TokenSource: that leaves
// the identity to deadrop's DefaultTokenSource, which reads whichever session
// is marked default rather than the one for --auth-server. Pointing
// --auth-server at a non-default environment would then present the default
// environment's identity to it, and the exchange fails with "no validator found
// for issuer". Sessions are already stored per server, so bind to that one.
func (so *ServerOptions) tokenSource() (credentials.TokenSource, error) {
	if so.AuthOptions.Server == "" {
		return nil, errors.New("auth server not set")
	}
	if len(so.AuthOptions.Audience) == 0 {
		return nil, errors.New("at least one audience is required")
	}

	// The env var keeps taking precedence, as with deadrop's default source.
	identity := credentials.NewChainedTokenSource(
		credentials.DefaultEnvTokenSource(),
		&renewingIdentitySource{serverURL: so.AuthOptions.Server},
	)

	opts := []credentials.ServiceTokenSourceOption{
		credentials.WithServiceIdentitySource(identity),
	}
	if !so.AuthOptions.DisablePersistence {
		opts = append(opts, credentials.WithServicePersistence())
	}

	return credentials.NewServiceTokenSource(
		&exchange.ExchangeRequest{Audience: so.AuthOptions.Audience},
		so.AuthOptions.Server,
		opts...,
	)
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
