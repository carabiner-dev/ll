// SPDX-FileCopyrightText: Copyright 2026 Carabiner Systems, Inc

package ll

import (
	"fmt"
	"strings"

	llv1 "github.com/carabiner-dev/ll/api/carabiner/ll/v1"
)

// EncodeID URL-encodes special characters in an ID that would conflict with tuple delimiters.
// Characters encoded: @ # : %
func EncodeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '%':
			b.WriteString("%25")
		case '@':
			b.WriteString("%40")
		case '#':
			b.WriteString("%23")
		case ':':
			b.WriteString("%3A")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// DecodeID URL-decodes an ID, reversing EncodeID.
func DecodeID(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			switch s[i+1 : i+3] {
			case "25":
				b.WriteByte('%')
				i += 2
			case "40":
				b.WriteByte('@')
				i += 2
			case "23":
				b.WriteByte('#')
				i += 2
			case "3A", "3a":
				b.WriteByte(':')
				i += 2
			default:
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// ValidateEncodedID checks that an ID is properly percent-encoded.
// Any '%' must be followed by exactly two hex digits.
func ValidateEncodedID(s string) error {
	for i := 0; i < len(s); i++ {
		if s[i] == '%' {
			if i+2 >= len(s) {
				return fmt.Errorf("invalid percent-encoding in %q: '%%' at position %d not followed by two hex digits", s, i)
			}
			if !isHexDigit(s[i+1]) || !isHexDigit(s[i+2]) {
				return fmt.Errorf("invalid percent-encoding in %q: '%%%c%c' is not valid", s, s[i+1], s[i+2])
			}
			i += 2 // skip the two hex digits
		}
	}
	return nil
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')
}

// ParseTuple parses a string like "document:doc1#viewer@user:alice" into a RelationTuple.
// Supports quoted values for IDs containing special characters:
//   - document:"doc@1"#viewer@user:alice
//   - document:doc1#viewer@user:"bob@email.com"
//
// Quoted values are URL-encoded in the resulting tuple.
// Already percent-encoded values are passed through as-is.
func ParseTuple(s string) (*llv1.RelationTuple, error) {
	// Format: object_type:object_id#relation@subject_type:subject_id[#subject_relation]
	//
	// We need to find delimiters (: # @) while respecting quoted sections.
	// Quoted sections use double quotes and their contents are URL-encoded.

	tokens, err := tokenize(s)
	if err != nil {
		return nil, err
	}

	// Expected structure: type : id # relation @ type : id [# relation]
	// Minimum tokens: type : id # relation @ type : id = 8 tokens
	if len(tokens) < 8 {
		return nil, fmt.Errorf("invalid tuple string %q: incomplete", s)
	}

	// Find the @ that separates object from subject (not inside a token)
	atIdx := -1
	for i, t := range tokens {
		if t == "@" {
			atIdx = i
			break
		}
	}
	if atIdx < 0 {
		return nil, fmt.Errorf("invalid tuple string %q: missing '@'", s)
	}

	objectTokens := tokens[:atIdx]
	subjectTokens := tokens[atIdx+1:]

	// Parse object part: type : id # relation
	objectType, objectID, relation, err := parseObjectTokens(objectTokens, s)
	if err != nil {
		return nil, err
	}

	// Parse subject part: type : id [# relation]
	subjectType, subjectID, subjectRelation, err := parseSubjectTokens(subjectTokens, s)
	if err != nil {
		return nil, err
	}

	return &llv1.RelationTuple{
		ObjectType:      objectType,
		ObjectId:        objectID,
		Relation:        relation,
		SubjectType:     subjectType,
		SubjectId:       subjectID,
		SubjectRelation: subjectRelation,
	}, nil
}

// ValidateTuple checks that a tuple has valid syntax and properly encoded IDs.
// It does not validate against a schema (that's done server-side).
func ValidateTuple(t *llv1.RelationTuple) error {
	if t == nil {
		return fmt.Errorf("tuple is nil")
	}
	if t.ObjectType == "" {
		return fmt.Errorf("empty object type")
	}
	if t.ObjectId == "" {
		return fmt.Errorf("empty object id")
	}
	if t.Relation == "" {
		return fmt.Errorf("empty relation")
	}
	if t.SubjectType == "" {
		return fmt.Errorf("empty subject type")
	}
	if t.SubjectId == "" {
		return fmt.Errorf("empty subject id")
	}

	// Validate ID encoding
	if err := ValidateEncodedID(t.ObjectId); err != nil {
		return fmt.Errorf("invalid object id: %w", err)
	}
	if err := ValidateEncodedID(t.SubjectId); err != nil {
		return fmt.Errorf("invalid subject id: %w", err)
	}

	return nil
}

// tokenize splits a tuple string into tokens, handling quoted sections.
// Quoted sections become single tokens with their contents URL-encoded.
// Unquoted tokens containing '%' are validated to ensure proper percent-encoding.
func tokenize(s string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if ch == '"' {
			if inQuote {
				// End of quoted section - encode the content
				tokens = append(tokens, EncodeID(current.String()))
				current.Reset()
				inQuote = false
			} else {
				// Start of quoted section - flush any current content
				if current.Len() > 0 {
					token := current.String()
					if err := ValidateEncodedID(token); err != nil {
						return nil, err
					}
					tokens = append(tokens, token)
					current.Reset()
				}
				inQuote = true
			}
			continue
		}

		if inQuote {
			current.WriteByte(ch)
			continue
		}

		// Outside quotes - check for delimiters
		if ch == ':' || ch == '#' || ch == '@' {
			if current.Len() > 0 {
				token := current.String()
				if err := ValidateEncodedID(token); err != nil {
					return nil, err
				}
				tokens = append(tokens, token)
				current.Reset()
			}
			tokens = append(tokens, string(ch))
		} else {
			current.WriteByte(ch)
		}
	}

	if inQuote {
		return nil, fmt.Errorf("unclosed quote in tuple string")
	}

	if current.Len() > 0 {
		token := current.String()
		if err := ValidateEncodedID(token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}

func parseObjectTokens(tokens []string, original string) (objectType, objectID, relation string, err error) {
	// Expected: type : id # relation
	// Find first : and first #
	colonIdx := -1
	hashIdx := -1
	for i, t := range tokens {
		if t == ":" && colonIdx < 0 {
			colonIdx = i
		}
		if t == "#" && hashIdx < 0 {
			hashIdx = i
		}
	}

	if colonIdx < 0 {
		return "", "", "", fmt.Errorf("invalid tuple string %q: missing ':' in object part", original)
	}
	if hashIdx < 0 {
		return "", "", "", fmt.Errorf("invalid tuple string %q: missing '#' in object part", original)
	}
	if colonIdx > hashIdx {
		return "", "", "", fmt.Errorf("invalid tuple string %q: ':' must come before '#' in object part", original)
	}

	// type is tokens before first :
	objectType = strings.Join(tokens[:colonIdx], "")
	// id is tokens between : and #
	objectID = strings.Join(tokens[colonIdx+1:hashIdx], "")
	// relation is tokens after #
	relation = strings.Join(tokens[hashIdx+1:], "")

	if objectType == "" {
		return "", "", "", fmt.Errorf("invalid tuple string %q: empty object type", original)
	}
	if objectID == "" {
		return "", "", "", fmt.Errorf("invalid tuple string %q: empty object id", original)
	}
	if relation == "" {
		return "", "", "", fmt.Errorf("invalid tuple string %q: empty relation", original)
	}

	return objectType, objectID, relation, nil
}

func parseSubjectTokens(tokens []string, original string) (subjectType, subjectID, subjectRelation string, err error) {
	// Expected: type : id [# relation]
	colonIdx := -1
	hashIdx := -1
	for i, t := range tokens {
		if t == ":" && colonIdx < 0 {
			colonIdx = i
		}
		if t == "#" && hashIdx < 0 {
			hashIdx = i
		}
	}

	if colonIdx < 0 {
		return "", "", "", fmt.Errorf("invalid tuple string %q: missing ':' in subject part", original)
	}

	subjectType = strings.Join(tokens[:colonIdx], "")

	if hashIdx >= 0 {
		// Has subject relation
		if hashIdx <= colonIdx {
			return "", "", "", fmt.Errorf("invalid tuple string %q: '#' must come after ':' in subject part", original)
		}
		subjectID = strings.Join(tokens[colonIdx+1:hashIdx], "")
		subjectRelation = strings.Join(tokens[hashIdx+1:], "")
	} else {
		subjectID = strings.Join(tokens[colonIdx+1:], "")
	}

	if subjectType == "" {
		return "", "", "", fmt.Errorf("invalid tuple string %q: empty subject type", original)
	}
	if subjectID == "" {
		return "", "", "", fmt.Errorf("invalid tuple string %q: empty subject id", original)
	}

	return subjectType, subjectID, subjectRelation, nil
}

// FormatTuple formats a RelationTuple as a string.
// IDs are stored URL-encoded, so this returns the encoded form.
func FormatTuple(t *llv1.RelationTuple) string {
	s := fmt.Sprintf("%s:%s#%s@%s:%s", t.ObjectType, t.ObjectId, t.Relation, t.SubjectType, t.SubjectId)
	if t.SubjectRelation != "" {
		s += "#" + t.SubjectRelation
	}
	return s
}

// FormatTupleDecoded formats a RelationTuple as a string with decoded IDs.
// This is useful for human-readable display. IDs containing special
// characters are wrapped in quotes.
func FormatTupleDecoded(t *llv1.RelationTuple) string {
	objectID := DecodeID(t.ObjectId)
	subjectID := DecodeID(t.SubjectId)

	// If decoded IDs contain special characters, wrap in quotes
	if needsQuoting(objectID) {
		objectID = fmt.Sprintf("%q", objectID)
	}
	if needsQuoting(subjectID) {
		subjectID = fmt.Sprintf("%q", subjectID)
	}

	s := fmt.Sprintf("%s:%s#%s@%s:%s", t.ObjectType, objectID, t.Relation, t.SubjectType, subjectID)
	if t.SubjectRelation != "" {
		s += "#" + t.SubjectRelation
	}
	return s
}

// needsQuoting returns true if the string contains characters that need quoting.
func needsQuoting(s string) bool {
	for _, r := range s {
		if r == '@' || r == '#' || r == ':' {
			return true
		}
	}
	return false
}
