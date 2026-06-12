// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

// Package ll provides a client for interacting with a Lamplight server.
package ll

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	llv1 "github.com/carabiner-dev/ll/api/carabiner/ll/v1"
)

// CallOption configures a single RPC call.
type CallOption func(*callOptions)

type callOptions struct {
	token string
}

// WithCallToken injects a bearer token for a single call, overriding the
// client-level token (set via WithToken at construction). Use it to forward an
// end user's identity per request, so the call is authorized as that user rather
// than as the client's own service identity. Intended for clients created
// without a baked-in token; pairing it with a client-level token would send two
// Authorization headers.
func WithCallToken(token string) CallOption {
	return func(o *callOptions) { o.token = token }
}

// applyCall threads per-call options into the outgoing context.
func applyCall(ctx context.Context, opts []CallOption) context.Context {
	var o callOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+o.token)
	}
	return ctx
}

// Client is the interface for interacting with a Lamplight server.
type Client interface {
	Check(ctx context.Context, tuple *llv1.RelationTuple, opts ...CallOption) (bool, error)
	Write(ctx context.Context, writes, deletes []*llv1.RelationTuple, opts ...CallOption) error
	Read(ctx context.Context, filter *llv1.RelationTupleFilter, opts ...CallOption) ([]*llv1.RelationTuple, error)
	Delete(ctx context.Context, filter *llv1.RelationTupleFilter, opts ...CallOption) error
	ListObjects(ctx context.Context, subjectType, subjectID, permission, objectType string, opts ...CallOption) ([]string, error)
	Expand(ctx context.Context, objectType, objectID, permission string, opts ...CallOption) (*llv1.ExpandTree, error)
	WriteSchema(ctx context.Context, yamlData string, opts ...CallOption) error
	ReadSchema(ctx context.Context, opts ...CallOption) (string, error)
	ReadSchemaSet(ctx context.Context, name string, opts ...CallOption) (string, error)
	ListSchemaSets(ctx context.Context, opts ...CallOption) ([]string, error)
	DeleteSchemaSet(ctx context.Context, name string, opts ...CallOption) error
	WhoAmI(ctx context.Context, opts ...CallOption) (*llv1.WhoAmIResponse, error)
	// EnsurePath ensures all parent tuples exist for the given path.
	// The path format is: type:id>type:id>type:id#relation
	// This creates tuples connecting each child to its parent using the specified relation.
	EnsurePath(ctx context.Context, path string, opts ...CallOption) error
	// CheckPath checks if all tuples in the given path exist.
	// Returns whether the path is complete and which tuples are found/missing.
	CheckPath(ctx context.Context, path string, opts ...CallOption) (*llv1.CheckPathResponse, error)
	// GrantRole assigns a role to a subject on an object.
	GrantRole(ctx context.Context, role, objectType, objectID, subjectType, subjectID string, opts ...CallOption) (*llv1.RoleAssignment, error)
	// RevokeRole removes a role assignment from a subject on an object.
	RevokeRole(ctx context.Context, role, objectType, objectID, subjectType, subjectID string, opts ...CallOption) error
	// ListRoleAssignments returns all role assignments matching the filter.
	ListRoleAssignments(ctx context.Context, objectType, objectID, subjectType, subjectID, role string, opts ...CallOption) ([]*llv1.RoleAssignment, error)
	// ListRoles returns the names of all defined roles.
	ListRoles(ctx context.Context, opts ...CallOption) ([]string, error)
	Close() error
}

// GRPCClient implements Client using gRPC.
type GRPCClient struct {
	conn     *grpc.ClientConn
	client   llv1.LamplightServiceClient
	token    string
	insecure bool
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

// WithInsecure disables TLS for the connection.
// By default, the client uses TLS for non-localhost addresses.
// Use this option for local development or when connecting to a server
// behind a TLS-terminating proxy on a trusted network.
func WithInsecure() Option {
	return func(c *GRPCClient) {
		c.insecure = true
	}
}

// New creates a new Client connected to the given server address.
// By default, TLS is used for non-localhost addresses. Use WithInsecure()
// to disable TLS for local development.
func New(serverAddr string, opts ...Option) (Client, error) {
	c := &GRPCClient{}

	// Apply options first to get the token and insecure flag
	for _, opt := range opts {
		opt(c)
	}

	// Determine if we should use TLS
	// Default to TLS unless:
	// 1. WithInsecure() was called
	// 2. Connecting to localhost
	useTLS := !c.insecure && !isLocalhost(serverAddr)

	// Build dial options
	var dialOpts []grpc.DialOption
	if useTLS {
		// Use system CA pool for TLS
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add per-RPC credentials if token is set
	if c.token != "" {
		dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(tokenAuth{token: c.token, requireTLS: useTLS}))
	}

	conn, err := grpc.NewClient(serverAddr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", serverAddr, err)
	}

	c.conn = conn
	c.client = llv1.NewLamplightServiceClient(conn)
	return c, nil
}

// isLocalhost returns true if the address points to localhost.
func isLocalhost(addr string) bool {
	// Strip port if present
	host := addr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		// Check if this is an IPv6 address [::1]:port
		if strings.HasPrefix(addr, "[") {
			if bracketIdx := strings.Index(addr, "]"); bracketIdx != -1 {
				host = addr[1:bracketIdx]
			}
		} else {
			host = addr[:idx]
		}
	}

	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "[::1]"
}

// tokenAuth implements grpc.PerRPCCredentials to add the auth token to each request.
type tokenAuth struct {
	token      string
	requireTLS bool
}

func (t tokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

func (t tokenAuth) RequireTransportSecurity() bool {
	return t.requireTLS
}

func (c *GRPCClient) Check(ctx context.Context, t *llv1.RelationTuple, opts ...CallOption) (bool, error) {
	ctx = applyCall(ctx, opts)
	if err := ValidateTuple(t); err != nil {
		return false, fmt.Errorf("invalid tuple: %w", err)
	}
	resp, err := c.client.Check(ctx, &llv1.CheckRequest{Tuple: t})
	if err != nil {
		return false, err
	}
	return resp.GetAllowed(), nil
}

func (c *GRPCClient) Write(ctx context.Context, writes, deletes []*llv1.RelationTuple, opts ...CallOption) error {
	ctx = applyCall(ctx, opts)
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

func (c *GRPCClient) Read(ctx context.Context, filter *llv1.RelationTupleFilter, opts ...CallOption) ([]*llv1.RelationTuple, error) {
	ctx = applyCall(ctx, opts)
	resp, err := c.client.Read(ctx, &llv1.ReadRequest{Filter: filter})
	if err != nil {
		return nil, err
	}
	return resp.GetTuples(), nil
}

func (c *GRPCClient) Delete(ctx context.Context, filter *llv1.RelationTupleFilter, opts ...CallOption) error {
	ctx = applyCall(ctx, opts)
	_, err := c.client.Delete(ctx, &llv1.DeleteRequest{Filter: filter})
	return err
}

func (c *GRPCClient) ListObjects(ctx context.Context, subjectType, subjectID, permission, objectType string, opts ...CallOption) ([]string, error) {
	ctx = applyCall(ctx, opts)
	resp, err := c.client.ListObjects(ctx, &llv1.ListObjectsRequest{
		SubjectType: subjectType,
		SubjectId:   subjectID,
		Permission:  permission,
		ObjectType:  objectType,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetObjectIds(), nil
}

func (c *GRPCClient) Expand(ctx context.Context, objectType, objectID, permission string, opts ...CallOption) (*llv1.ExpandTree, error) {
	ctx = applyCall(ctx, opts)
	resp, err := c.client.Expand(ctx, &llv1.ExpandRequest{
		ObjectType: objectType,
		ObjectId:   objectID,
		Permission: permission,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetTree(), nil
}

func (c *GRPCClient) WriteSchema(ctx context.Context, yamlData string, opts ...CallOption) error {
	ctx = applyCall(ctx, opts)
	_, err := c.client.WriteSchema(ctx, &llv1.WriteSchemaRequest{SchemaYaml: yamlData})
	return err
}

func (c *GRPCClient) ReadSchema(ctx context.Context, opts ...CallOption) (string, error) {
	ctx = applyCall(ctx, opts)
	resp, err := c.client.ReadSchema(ctx, &llv1.ReadSchemaRequest{})
	if err != nil {
		return "", err
	}
	return resp.GetSchemaYaml(), nil
}

func (c *GRPCClient) ReadSchemaSet(ctx context.Context, name string, opts ...CallOption) (string, error) {
	ctx = applyCall(ctx, opts)
	resp, err := c.client.ReadSchema(ctx, &llv1.ReadSchemaRequest{Name: name})
	if err != nil {
		return "", err
	}
	return resp.GetSchemaYaml(), nil
}

func (c *GRPCClient) ListSchemaSets(ctx context.Context, opts ...CallOption) ([]string, error) {
	ctx = applyCall(ctx, opts)
	resp, err := c.client.ListSchemaSets(ctx, &llv1.ListSchemaSetsRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetNames(), nil
}

func (c *GRPCClient) DeleteSchemaSet(ctx context.Context, name string, opts ...CallOption) error {
	ctx = applyCall(ctx, opts)
	_, err := c.client.DeleteSchemaSet(ctx, &llv1.DeleteSchemaSetRequest{Name: name})
	return err
}

func (c *GRPCClient) WhoAmI(ctx context.Context, opts ...CallOption) (*llv1.WhoAmIResponse, error) {
	ctx = applyCall(ctx, opts)
	return c.client.WhoAmI(ctx, &llv1.WhoAmIRequest{})
}

func (c *GRPCClient) EnsurePath(ctx context.Context, pathStr string, opts ...CallOption) error {
	ctx = applyCall(ctx, opts)
	_, err := c.client.EnsurePath(ctx, &llv1.EnsurePathRequest{Path: pathStr})
	return err
}

func (c *GRPCClient) CheckPath(ctx context.Context, pathStr string, opts ...CallOption) (*llv1.CheckPathResponse, error) {
	ctx = applyCall(ctx, opts)
	return c.client.CheckPath(ctx, &llv1.CheckPathRequest{Path: pathStr})
}

func (c *GRPCClient) GrantRole(ctx context.Context, role, objectType, objectID, subjectType, subjectID string, opts ...CallOption) (*llv1.RoleAssignment, error) {
	ctx = applyCall(ctx, opts)
	resp, err := c.client.GrantRole(ctx, &llv1.GrantRoleRequest{
		Role:        role,
		ObjectType:  objectType,
		ObjectId:    objectID,
		SubjectType: subjectType,
		SubjectId:   subjectID,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetAssignment(), nil
}

func (c *GRPCClient) RevokeRole(ctx context.Context, role, objectType, objectID, subjectType, subjectID string, opts ...CallOption) error {
	ctx = applyCall(ctx, opts)
	_, err := c.client.RevokeRole(ctx, &llv1.RevokeRoleRequest{
		Role:        role,
		ObjectType:  objectType,
		ObjectId:    objectID,
		SubjectType: subjectType,
		SubjectId:   subjectID,
	})
	return err
}

func (c *GRPCClient) ListRoleAssignments(ctx context.Context, objectType, objectID, subjectType, subjectID, role string, opts ...CallOption) ([]*llv1.RoleAssignment, error) {
	ctx = applyCall(ctx, opts)
	resp, err := c.client.ListRoleAssignments(ctx, &llv1.ListRoleAssignmentsRequest{
		ObjectType:  objectType,
		ObjectId:    objectID,
		SubjectType: subjectType,
		SubjectId:   subjectID,
		Role:        role,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetAssignments(), nil
}

func (c *GRPCClient) ListRoles(ctx context.Context, opts ...CallOption) ([]string, error) {
	ctx = applyCall(ctx, opts)
	resp, err := c.client.ListRoles(ctx, &llv1.ListRolesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.GetRoles(), nil
}

func (c *GRPCClient) Close() error {
	return c.conn.Close()
}
