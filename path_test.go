// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc
// SPDX-License-Identifier: Apache-2.0

package ll

import (
	"testing"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantComps int
		wantRel   string
	}{
		{
			name:      "simple two-level path",
			input:     "folder:root>file:doc.txt#parent",
			wantComps: 2,
			wantRel:   "parent",
		},
		{
			name:      "three-level path",
			input:     "folder:root>folder:sub>file:doc.txt#parent",
			wantComps: 3,
			wantRel:   "parent",
		},
		{
			name:      "org and repo",
			input:     "org:acme>repo:backend#owner",
			wantComps: 2,
			wantRel:   "owner",
		},
		{
			name:      "with encoded characters",
			input:     "folder:my%3Efolder>file:doc%23test.txt#parent",
			wantComps: 2,
			wantRel:   "parent",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no relation",
			input:   "folder:root>file:doc.txt",
			wantErr: true,
		},
		{
			name:    "empty relation",
			input:   "folder:root>file:doc.txt#",
			wantErr: true,
		},
		{
			name:    "single component",
			input:   "folder:root#parent",
			wantErr: true,
		},
		{
			name:    "missing type",
			input:   ":root>file:doc.txt#parent",
			wantErr: true,
		},
		{
			name:    "missing id",
			input:   "folder:>file:doc.txt#parent",
			wantErr: true,
		},
		{
			name:    "empty component",
			input:   "folder:root>>file:doc.txt#parent",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ParsePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParsePath(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParsePath(%q) unexpected error: %v", tt.input, err)
				return
			}
			if len(path.Components) != tt.wantComps {
				t.Errorf("ParsePath(%q) got %d components, want %d", tt.input, len(path.Components), tt.wantComps)
			}
			if path.Relation != tt.wantRel {
				t.Errorf("ParsePath(%q) got relation %q, want %q", tt.input, path.Relation, tt.wantRel)
			}
		})
	}
}

func TestPathTuples(t *testing.T) {
	path, err := ParsePath("folder:root>folder:sub>file:doc.txt#parent")
	if err != nil {
		t.Fatalf("ParsePath failed: %v", err)
	}

	tuples := path.Tuples()
	if len(tuples) != 2 {
		t.Fatalf("expected 2 tuples, got %d", len(tuples))
	}

	// First tuple: folder:sub#parent@folder:root
	if tuples[0].GetObjectType() != "folder" || tuples[0].GetObjectId() != "sub" {
		t.Errorf("tuple[0] object = %s:%s, want folder:sub", tuples[0].GetObjectType(), tuples[0].GetObjectId())
	}
	if tuples[0].GetRelation() != "parent" {
		t.Errorf("tuple[0] relation = %s, want parent", tuples[0].GetRelation())
	}
	if tuples[0].GetSubjectType() != "folder" || tuples[0].GetSubjectId() != "root" {
		t.Errorf("tuple[0] subject = %s:%s, want folder:root", tuples[0].GetSubjectType(), tuples[0].GetSubjectId())
	}

	// Second tuple: file:doc.txt#parent@folder:sub
	if tuples[1].GetObjectType() != "file" || tuples[1].GetObjectId() != "doc.txt" {
		t.Errorf("tuple[1] object = %s:%s, want file:doc.txt", tuples[1].GetObjectType(), tuples[1].GetObjectId())
	}
	if tuples[1].GetRelation() != "parent" {
		t.Errorf("tuple[1] relation = %s, want parent", tuples[1].GetRelation())
	}
	if tuples[1].GetSubjectType() != "folder" || tuples[1].GetSubjectId() != "sub" {
		t.Errorf("tuple[1] subject = %s:%s, want folder:sub", tuples[1].GetSubjectType(), tuples[1].GetSubjectId())
	}
}

func TestPathString(t *testing.T) {
	tests := []struct {
		name string
		path *Path
		want string
	}{
		{
			name: "simple path",
			path: &Path{
				Components: []PathComponent{
					{Type: "folder", ID: "root"},
					{Type: "file", ID: "doc.txt"},
				},
				Relation: "parent",
			},
			want: "folder:root>file:doc.txt#parent",
		},
		{
			name: "with special characters",
			path: &Path{
				Components: []PathComponent{
					{Type: "folder", ID: "my>folder"},
					{Type: "file", ID: "doc#test.txt"},
				},
				Relation: "parent",
			},
			want: "folder:my%3Efolder>file:doc%23test.txt#parent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.path.String()
			if got != tt.want {
				t.Errorf("Path.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPathRoundTrip(t *testing.T) {
	original := "folder:root>folder:projects>file:readme.md#parent"

	path, err := ParsePath(original)
	if err != nil {
		t.Fatalf("ParsePath failed: %v", err)
	}

	result := path.String()
	if result != original {
		t.Errorf("round trip failed: got %q, want %q", result, original)
	}
}

func TestBuildPath(t *testing.T) {
	path := BuildPath("org", "acme").
		Child("team", "backend").
		Child("repo", "api").
		Relation("owner")

	if len(path.Components) != 3 {
		t.Errorf("expected 3 components, got %d", len(path.Components))
	}
	if path.Relation != "owner" {
		t.Errorf("expected relation 'owner', got %q", path.Relation)
	}

	expected := "org:acme>team:backend>repo:api#owner"
	if path.String() != expected {
		t.Errorf("expected %q, got %q", expected, path.String())
	}
}

func TestPathLeafAndRoot(t *testing.T) {
	path, _ := ParsePath("folder:root>folder:sub>file:doc.txt#parent")

	root := path.Root()
	if root.Type != "folder" || root.ID != "root" {
		t.Errorf("Root() = %s:%s, want folder:root", root.Type, root.ID)
	}

	leaf := path.Leaf()
	if leaf.Type != "file" || leaf.ID != "doc.txt" {
		t.Errorf("Leaf() = %s:%s, want file:doc.txt", leaf.Type, leaf.ID)
	}
}
