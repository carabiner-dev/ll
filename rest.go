// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ll

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	llv1 "github.com/carabiner-dev/ll/api/carabiner/ll/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// RESTClient implements Client using HTTP/REST.
type RESTClient struct {
	baseURL     string
	httpClient  *http.Client
	token       string
	insecure    bool
	marshaler   protojson.MarshalOptions
	unmarshaler protojson.UnmarshalOptions
}

// RESTOption configures the REST client.
type RESTOption func(*RESTClient)

// WithRESTToken sets the authentication token for the REST client.
func WithRESTToken(token string) RESTOption {
	return func(c *RESTClient) {
		c.token = token
	}
}

// WithRESTInsecure forces HTTP instead of HTTPS for non-localhost addresses.
func WithRESTInsecure() RESTOption {
	return func(c *RESTClient) {
		c.insecure = true
	}
}

// NewREST creates a new REST Client connected to the given server address.
// By default, HTTPS is used for non-localhost addresses. Use WithRESTInsecure()
// to force HTTP for local development.
func NewREST(serverAddr string, opts ...RESTOption) (Client, error) {
	c := &RESTClient{
		httpClient:  http.DefaultClient,
		marshaler:   protojson.MarshalOptions{},
		unmarshaler: protojson.UnmarshalOptions{DiscardUnknown: true},
	}

	// Apply options first to get insecure flag
	for _, opt := range opts {
		opt(c)
	}

	// Determine scheme based on address and insecure flag
	if serverAddr != "" && !strings.HasPrefix(serverAddr, "http://") && !strings.HasPrefix(serverAddr, "https://") {
		if c.insecure || isLocalhost(serverAddr) {
			serverAddr = "http://" + serverAddr
		} else {
			serverAddr = "https://" + serverAddr
		}
	}

	c.baseURL = serverAddr
	return c, nil
}

func (c *RESTClient) doRequest(ctx context.Context, method, path string, reqBody, respBody proto.Message) error {
	var bodyReader io.Reader
	if reqBody != nil {
		data, err := c.marshaler.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, string(respData))
	}

	if respBody != nil {
		if err := c.unmarshaler.Unmarshal(respData, respBody); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}

	return nil
}

func (c *RESTClient) Check(ctx context.Context, t *llv1.RelationTuple) (bool, error) {
	if err := ValidateTuple(t); err != nil {
		return false, fmt.Errorf("invalid tuple: %w", err)
	}

	req := &llv1.CheckRequest{Tuple: t}
	resp := &llv1.CheckResponse{}

	if err := c.doRequest(ctx, "POST", "/v1/check", req, resp); err != nil {
		return false, err
	}

	return resp.Allowed, nil
}

func (c *RESTClient) Write(ctx context.Context, writes, deletes []*llv1.RelationTuple) error {
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

	req := &llv1.WriteRequest{Writes: writes, Deletes: deletes}
	resp := &llv1.WriteResponse{}

	return c.doRequest(ctx, "POST", "/v1/write", req, resp)
}

func (c *RESTClient) Read(ctx context.Context, filter *llv1.RelationTupleFilter) ([]*llv1.RelationTuple, error) {
	req := &llv1.ReadRequest{Filter: filter}
	resp := &llv1.ReadResponse{}

	if err := c.doRequest(ctx, "POST", "/v1/read", req, resp); err != nil {
		return nil, err
	}

	return resp.Tuples, nil
}

func (c *RESTClient) Delete(ctx context.Context, filter *llv1.RelationTupleFilter) error {
	req := &llv1.DeleteRequest{Filter: filter}
	resp := &llv1.DeleteResponse{}

	return c.doRequest(ctx, "POST", "/v1/delete", req, resp)
}

func (c *RESTClient) ListObjects(ctx context.Context, subjectType, subjectID, permission, objectType string) ([]string, error) {
	req := &llv1.ListObjectsRequest{
		SubjectType: subjectType,
		SubjectId:   subjectID,
		Permission:  permission,
		ObjectType:  objectType,
	}
	resp := &llv1.ListObjectsResponse{}

	if err := c.doRequest(ctx, "POST", "/v1/list-objects", req, resp); err != nil {
		return nil, err
	}

	return resp.ObjectIds, nil
}

func (c *RESTClient) Expand(ctx context.Context, objectType, objectID, permission string) (*llv1.ExpandTree, error) {
	req := &llv1.ExpandRequest{
		ObjectType: objectType,
		ObjectId:   objectID,
		Permission: permission,
	}
	resp := &llv1.ExpandResponse{}

	if err := c.doRequest(ctx, "POST", "/v1/expand", req, resp); err != nil {
		return nil, err
	}

	return resp.Tree, nil
}

func (c *RESTClient) WriteSchema(ctx context.Context, yamlData string) error {
	req := &llv1.WriteSchemaRequest{SchemaYaml: yamlData}
	resp := &llv1.WriteSchemaResponse{}

	return c.doRequest(ctx, "POST", "/v1/schema", req, resp)
}

func (c *RESTClient) ReadSchema(ctx context.Context) (string, error) {
	resp := &llv1.ReadSchemaResponse{}

	if err := c.doRequest(ctx, "GET", "/v1/schema", nil, resp); err != nil {
		return "", err
	}

	return resp.SchemaYaml, nil
}

func (c *RESTClient) ReadSchemaSet(ctx context.Context, name string) (string, error) {
	resp := &llv1.ReadSchemaResponse{}

	if err := c.doRequest(ctx, "GET", "/v1/schema/"+name, nil, resp); err != nil {
		return "", err
	}

	return resp.SchemaYaml, nil
}

func (c *RESTClient) ListSchemaSets(ctx context.Context) ([]string, error) {
	resp := &llv1.ListSchemaSetsResponse{}

	if err := c.doRequest(ctx, "GET", "/v1/schema/sets", nil, resp); err != nil {
		return nil, err
	}

	return resp.Names, nil
}

func (c *RESTClient) DeleteSchemaSet(ctx context.Context, name string) error {
	resp := &llv1.DeleteSchemaSetResponse{}

	return c.doRequest(ctx, "DELETE", "/v1/schema/"+name, nil, resp)
}

func (c *RESTClient) WhoAmI(ctx context.Context) (*llv1.WhoAmIResponse, error) {
	resp := &llv1.WhoAmIResponse{}

	if err := c.doRequest(ctx, "GET", "/v1/whoami", nil, resp); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *RESTClient) EnsurePath(ctx context.Context, pathStr string) error {
	req := &llv1.EnsurePathRequest{Path: pathStr}
	resp := &llv1.EnsurePathResponse{}

	return c.doRequest(ctx, "POST", "/v1/ensure-path", req, resp)
}

func (c *RESTClient) GrantRole(ctx context.Context, role, objectType, objectID, subjectType, subjectID string) (*llv1.RoleAssignment, error) {
	req := &llv1.GrantRoleRequest{
		Role:        role,
		ObjectType:  objectType,
		ObjectId:    objectID,
		SubjectType: subjectType,
		SubjectId:   subjectID,
	}
	resp := &llv1.GrantRoleResponse{}

	if err := c.doRequest(ctx, "POST", "/v1/role/grant", req, resp); err != nil {
		return nil, err
	}

	return resp.Assignment, nil
}

func (c *RESTClient) RevokeRole(ctx context.Context, role, objectType, objectID, subjectType, subjectID string) error {
	req := &llv1.RevokeRoleRequest{
		Role:        role,
		ObjectType:  objectType,
		ObjectId:    objectID,
		SubjectType: subjectType,
		SubjectId:   subjectID,
	}
	resp := &llv1.RevokeRoleResponse{}

	return c.doRequest(ctx, "POST", "/v1/role/revoke", req, resp)
}

func (c *RESTClient) ListRoleAssignments(ctx context.Context, objectType, objectID, subjectType, subjectID, role string) ([]*llv1.RoleAssignment, error) {
	req := &llv1.ListRoleAssignmentsRequest{
		ObjectType:  objectType,
		ObjectId:    objectID,
		SubjectType: subjectType,
		SubjectId:   subjectID,
		Role:        role,
	}
	resp := &llv1.ListRoleAssignmentsResponse{}

	if err := c.doRequest(ctx, "POST", "/v1/role/list", req, resp); err != nil {
		return nil, err
	}

	return resp.Assignments, nil
}

func (c *RESTClient) ListRoles(ctx context.Context) ([]string, error) {
	resp := &llv1.ListRolesResponse{}

	if err := c.doRequest(ctx, "GET", "/v1/roles", nil, resp); err != nil {
		return nil, err
	}

	return resp.Roles, nil
}

func (c *RESTClient) Close() error {
	// HTTP client doesn't need explicit closing
	return nil
}
