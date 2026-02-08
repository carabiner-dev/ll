// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

// Package ll provides a client for interacting with a Lamplight server.
package ll

import (
	"context"
	"fmt"

	llv1 "github.com/carabiner-dev/ll/api/carabiner/ll/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is the interface for interacting with a Lamplight server.
type Client interface {
	Check(ctx context.Context, tuple *llv1.RelationTuple) (bool, error)
	Write(ctx context.Context, writes, deletes []*llv1.RelationTuple) error
	Read(ctx context.Context, filter *llv1.RelationTupleFilter) ([]*llv1.RelationTuple, error)
	Delete(ctx context.Context, filter *llv1.RelationTupleFilter) error
	ListObjects(ctx context.Context, subjectType, subjectID, permission, objectType string) ([]string, error)
	Expand(ctx context.Context, objectType, objectID, permission string) (*llv1.ExpandTree, error)
	WriteSchema(ctx context.Context, yamlData string) error
	ReadSchema(ctx context.Context) (string, error)
	ReadSchemaSet(ctx context.Context, name string) (string, error)
	ListSchemaSets(ctx context.Context) ([]string, error)
	DeleteSchemaSet(ctx context.Context, name string) error
	Close() error
}

// GRPCClient implements Client using gRPC.
type GRPCClient struct {
	conn   *grpc.ClientConn
	client llv1.LamplightServiceClient
	token  string
}

// Option configures the client.
type Option func(*GRPCClient)

// WithToken sets the authentication token for the client.
// The token will be sent as a Bearer token in the Authorization header.
func WithToken(token string) Option {
	return func(c *GRPCClient) {
		c.token = token
	}
}

// New creates a new Client connected to the given server address.
func New(serverAddr string, opts ...Option) (Client, error) {
	c := &GRPCClient{}

	// Apply options first to get the token
	for _, opt := range opts {
		opt(c)
	}

	// Build dial options
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Add per-RPC credentials if token is set
	if c.token != "" {
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(tokenAuth{token: c.token}))
	}

	conn, err := grpc.NewClient(serverAddr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", serverAddr, err)
	}

	c.conn = conn
	c.client = llv1.NewLamplightServiceClient(conn)
	return c, nil
}

// tokenAuth implements grpc.PerRPCCredentials to add the auth token to each request.
type tokenAuth struct {
	token string
}

func (t tokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

func (t tokenAuth) RequireTransportSecurity() bool {
	// Return false to allow insecure connections (for local development).
	// In production, this should ideally require TLS.
	return false
}

func (c *GRPCClient) Check(ctx context.Context, t *llv1.RelationTuple) (bool, error) {
	if err := ValidateTuple(t); err != nil {
		return false, fmt.Errorf("invalid tuple: %w", err)
	}
	resp, err := c.client.Check(ctx, &llv1.CheckRequest{Tuple: t})
	if err != nil {
		return false, err
	}
	return resp.Allowed, nil
}

func (c *GRPCClient) Write(ctx context.Context, writes, deletes []*llv1.RelationTuple) error {
	// Validate all tuples before sending
	for i, t := range writes {
		if err := ValidateTuple(t); err != nil {
			return fmt.Errorf("invalid write tuple[%d]: %w", i, err)
		}
	}
	for i, t := range deletes {
		if err := ValidateTuple(t); err != nil {
			return fmt.Errorf("invalid delete tuple[%d]: %w", i, err)
		}
	}
	_, err := c.client.Write(ctx, &llv1.WriteRequest{Writes: writes, Deletes: deletes})
	return err
}

func (c *GRPCClient) Read(ctx context.Context, filter *llv1.RelationTupleFilter) ([]*llv1.RelationTuple, error) {
	resp, err := c.client.Read(ctx, &llv1.ReadRequest{Filter: filter})
	if err != nil {
		return nil, err
	}
	return resp.Tuples, nil
}

func (c *GRPCClient) Delete(ctx context.Context, filter *llv1.RelationTupleFilter) error {
	_, err := c.client.Delete(ctx, &llv1.DeleteRequest{Filter: filter})
	return err
}

func (c *GRPCClient) ListObjects(ctx context.Context, subjectType, subjectID, permission, objectType string) ([]string, error) {
	resp, err := c.client.ListObjects(ctx, &llv1.ListObjectsRequest{
		SubjectType: subjectType,
		SubjectId:   subjectID,
		Permission:  permission,
		ObjectType:  objectType,
	})
	if err != nil {
		return nil, err
	}
	return resp.ObjectIds, nil
}

func (c *GRPCClient) Expand(ctx context.Context, objectType, objectID, permission string) (*llv1.ExpandTree, error) {
	resp, err := c.client.Expand(ctx, &llv1.ExpandRequest{
		ObjectType: objectType,
		ObjectId:   objectID,
		Permission: permission,
	})
	if err != nil {
		return nil, err
	}
	return resp.Tree, nil
}

func (c *GRPCClient) WriteSchema(ctx context.Context, yamlData string) error {
	_, err := c.client.WriteSchema(ctx, &llv1.WriteSchemaRequest{SchemaYaml: yamlData})
	return err
}

func (c *GRPCClient) ReadSchema(ctx context.Context) (string, error) {
	resp, err := c.client.ReadSchema(ctx, &llv1.ReadSchemaRequest{})
	if err != nil {
		return "", err
	}
	return resp.SchemaYaml, nil
}

func (c *GRPCClient) ReadSchemaSet(ctx context.Context, name string) (string, error) {
	resp, err := c.client.ReadSchema(ctx, &llv1.ReadSchemaRequest{Name: name})
	if err != nil {
		return "", err
	}
	return resp.SchemaYaml, nil
}

func (c *GRPCClient) ListSchemaSets(ctx context.Context) ([]string, error) {
	resp, err := c.client.ListSchemaSets(ctx, &llv1.ListSchemaSetsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.Names, nil
}

func (c *GRPCClient) DeleteSchemaSet(ctx context.Context, name string) error {
	_, err := c.client.DeleteSchemaSet(ctx, &llv1.DeleteSchemaSetRequest{Name: name})
	return err
}

func (c *GRPCClient) Close() error {
	return c.conn.Close()
}
