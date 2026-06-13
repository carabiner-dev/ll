// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ll

import (
	"fmt"
	"strings"

	llv1 "github.com/carabiner-dev/ll/api/carabiner/ll/v1"
)

// Path represents a hierarchical path of objects connected by a relation.
// The path syntax is: type:id>type:id>type:id#relation
//
// Example: folder:root>folder:projects>file:doc.txt#parent
//
// This represents a hierarchy where:
//   - folder:projects has parent folder:root
//   - file:doc.txt has parent folder:projects
//
// Special characters in object IDs should be URL-encoded:
//   - > as %3E
//   - # as %23
//   - : as %3A (only in the ID portion, after type)
type Path struct {
	// Components are the objects in the path, ordered from root to leaf.
	Components []PathComponent

	// Relation is the relation that connects parent to child.
	Relation string
}

// PathComponent represents a single object in a path.
type PathComponent struct {
	Type string
	ID   string
}

// ParsePath parses a path string into a Path struct.
//
// Format: type:id>type:id>type:id#relation
//
// Examples:
//
//	folder:root>folder:sub>file:doc.txt#parent
//	org:acme>repo:backend#owner
func ParsePath(s string) (*Path, error) {
	if s == "" {
		return nil, fmt.Errorf("empty path")
	}

	// Split off the relation (after #)
	hashIdx := strings.LastIndex(s, "#")
	if hashIdx == -1 {
		return nil, fmt.Errorf("path must include relation after #: %s", s)
	}

	pathPart := s[:hashIdx]
	relation := s[hashIdx+1:]

	if relation == "" {
		return nil, fmt.Errorf("empty relation in path: %s", s)
	}

	if pathPart == "" {
		return nil, fmt.Errorf("empty path components: %s", s)
	}

	// Split path into components
	parts := strings.Split(pathPart, ">")
	if len(parts) < 2 {
		return nil, fmt.Errorf("path must have at least 2 components: %s", s)
	}

	components := make([]PathComponent, 0, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty component at position %d: %s", i, s)
		}

		// Parse type:id
		colonIdx := strings.Index(part, ":")
		if colonIdx == -1 {
			return nil, fmt.Errorf("component %d missing type:id format: %s", i, part)
		}

		objType := part[:colonIdx]
		objID := part[colonIdx+1:]

		if objType == "" {
			return nil, fmt.Errorf("empty type in component %d: %s", i, part)
		}
		if objID == "" {
			return nil, fmt.Errorf("empty id in component %d: %s", i, part)
		}

		// Decode URL-encoded characters
		objType = DecodeID(objType)
		objID = DecodeID(objID)

		components = append(components, PathComponent{
			Type: objType,
			ID:   objID,
		})
	}

	return &Path{
		Components: components,
		Relation:   relation,
	}, nil
}

// String returns the path as a string.
func (p *Path) String() string {
	if p == nil || len(p.Components) == 0 {
		return ""
	}

	var b strings.Builder
	for i, c := range p.Components {
		if i > 0 {
			b.WriteByte('>')
		}
		// Encode special characters in type and id
		b.WriteString(EncodeID(c.Type))
		b.WriteByte(':')
		b.WriteString(EncodeID(c.ID))
	}
	b.WriteByte('#')
	b.WriteString(p.Relation)
	return b.String()
}

// Tuples returns the relation tuples that establish this path.
// Each tuple connects a child to its parent using the path's relation.
//
// For path folder:root>folder:sub>file:doc#parent, returns:
//   - folder:sub#parent@folder:root
//   - file:doc#parent@folder:sub
func (p *Path) Tuples() []*llv1.RelationTuple {
	if p == nil || len(p.Components) < 2 {
		return nil
	}

	tuples := make([]*llv1.RelationTuple, 0, len(p.Components)-1)
	for i := 1; i < len(p.Components); i++ {
		parent := p.Components[i-1]
		child := p.Components[i]

		tuples = append(tuples, &llv1.RelationTuple{
			ObjectType:  child.Type,
			ObjectId:    child.ID,
			Relation:    p.Relation,
			SubjectType: parent.Type,
			SubjectId:   parent.ID,
		})
	}
	return tuples
}

// Leaf returns the last component in the path (the deepest child).
func (p *Path) Leaf() *PathComponent {
	if p == nil || len(p.Components) == 0 {
		return nil
	}
	return &p.Components[len(p.Components)-1]
}

// Root returns the first component in the path (the topmost parent).
func (p *Path) Root() *PathComponent {
	if p == nil || len(p.Components) == 0 {
		return nil
	}
	return &p.Components[0]
}

// NewPath creates a new Path from components and a relation.
func NewPath(relation string, components ...PathComponent) *Path {
	return &Path{
		Components: components,
		Relation:   relation,
	}
}

// PathBuilder provides a fluent API for building paths.
type PathBuilder struct {
	components []PathComponent
}

// BuildPath starts building a new path with the root object.
func BuildPath(objType, objID string) *PathBuilder {
	return &PathBuilder{
		components: []PathComponent{{Type: objType, ID: objID}},
	}
}

// Child adds a child component to the path.
func (b *PathBuilder) Child(objType, objID string) *PathBuilder {
	b.components = append(b.components, PathComponent{Type: objType, ID: objID})
	return b
}

// Relation sets the relation that connects components and returns the built Path.
func (b *PathBuilder) Relation(relation string) *Path {
	return &Path{
		Components: b.components,
		Relation:   relation,
	}
}
