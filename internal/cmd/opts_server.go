// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/carabiner-dev/command"
	"github.com/spf13/cobra"
)

var _ command.OptionsSet = (*ServerOptions)(nil)

var defaultServerOptions = ServerOptions{
	Server: "localhost:8080",
}

// ServerOptions contains the common server connection options.
type ServerOptions struct {
	Server string
	Token  string
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
