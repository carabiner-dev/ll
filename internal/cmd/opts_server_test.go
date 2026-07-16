// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package cmd

import "testing"

// TestAudienceFromServer pins the audience the exchange requests: the full
// service URL of --server, matching what the platform's exchangers allowlist
// (e.g. "https://lamplight.dev.carabiner.dev"). Regression guard for the bare
// "lamplight" audience, which no environment accepts.
func TestAudienceFromServer(t *testing.T) {
	for _, tc := range []struct {
		name     string
		server   string
		insecure bool
		want     string
		mustErr  bool
	}{
		{name: "dev url", server: "https://lamplight.dev.carabiner.dev", want: "https://lamplight.dev.carabiner.dev"},
		{name: "prod url", server: "https://lamplight.carabiner.dev", want: "https://lamplight.carabiner.dev"},
		{name: "path and query are dropped", server: "https://lamplight.carabiner.dev/v1/x?a=b", want: "https://lamplight.carabiner.dev"},
		{name: "trailing slash", server: "https://lamplight.carabiner.dev/", want: "https://lamplight.carabiner.dev"},
		{name: "surrounding space", server: "  https://lamplight.carabiner.dev  ", want: "https://lamplight.carabiner.dev"},
		{name: "port is kept", server: "https://lamplight.carabiner.dev:8443", want: "https://lamplight.carabiner.dev:8443"},

		// --server doubles as a gRPC dial target, so bare host:port is valid.
		{name: "bare host defaults to https", server: "lamplight.dev.carabiner.dev", want: "https://lamplight.dev.carabiner.dev"},
		{name: "localhost dials clear", server: "localhost:8080", want: "http://localhost:8080"},
		{name: "loopback ip dials clear", server: "127.0.0.1:8080", want: "http://127.0.0.1:8080"},
		{name: "insecure dials clear", server: "lamplight.internal:8080", insecure: true, want: "http://lamplight.internal:8080"},

		{name: "empty errors", server: "", mustErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			so := ServerOptions{Server: tc.server, Insecure: tc.insecure}
			got, err := so.audience()
			if tc.mustErr {
				if err == nil {
					t.Fatalf("audience(%q) = %q, want an error", tc.server, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("audience(%q): %v", tc.server, err)
			}
			if got != tc.want {
				t.Errorf("audience(%q) = %q, want %q", tc.server, got, tc.want)
			}
		})
	}
}
