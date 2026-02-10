# SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
# SPDX-License-Identifier: Apache-2.0

.PHONY: proto build test clean

# Version can be set via environment or git tag
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

proto:
	buf lint
	buf generate

build: proto
	go build ./...
	go build -ldflags="-X main.version=$(VERSION)" -o bin/llctl ./cmd/llctl/

test:
	go test ./...

clean:
	rm -rf bin/
	rm -rf api/carabiner/
