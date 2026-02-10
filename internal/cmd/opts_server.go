// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/carabiner-dev/command"
	"github.com/carabiner-dev/ll"
	"github.com/spf13/cobra"
)

var _ command.OptionsSet = (*ServerOptions)(nil)

var defaultServerOptions = ServerOptions{
	Server: "localhost:8080",
}

// ServerOptions contains the common server connection options.
type ServerOptions struct {
	Server   string
	Token    string
	UseREST  bool
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
	cmd.PersistentFlags().StringVar(&so.Token, "token", "", "Authentication token (file path or token string)")
	cmd.PersistentFlags().BoolVar(&so.UseREST, "rest", false, "Use REST API instead of gRPC")
	cmd.PersistentFlags().BoolVar(&so.Insecure, "insecure", false, "Disable TLS (for local development)")
}

// GetToken returns the authentication token.
// If Token looks like a file path and the file exists, it reads the token from the file.
// Otherwise, it returns Token as-is.
func (so *ServerOptions) GetToken() (string, error) {
	if so.Token == "" {
		return "", nil
	}

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

// NewClient creates a new Lamplight client based on the configured options.
// If UseREST is true, it creates a REST client; otherwise, it creates a gRPC client.
func (so *ServerOptions) NewClient() (ll.Client, error) {
	token, err := so.GetToken()
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
