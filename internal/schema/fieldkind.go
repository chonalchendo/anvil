package schema

import (
	"encoding/json"
	"fmt"
)

// Kind classifies a top-level frontmatter field by its JSON Schema type.
type Kind int

const (
	KindUnknown Kind = iota
	KindScalar
	KindArray
	KindObject
)

// FieldKind reports the shape of typeName's fieldName per the embedded schema.
// KindUnknown means the field is not declared on the schema (ad-hoc field).
// An error is returned only when typeName itself has no embedded schema.
func FieldKind(typeName, fieldName string) (Kind, error) {
	b, err := EmbeddedFS.ReadFile(typeName + ".schema.json")
	if err != nil {
		return KindUnknown, fmt.Errorf("read %s schema: %w", typeName, err)
	}
	var raw struct {
		Properties map[string]struct {
			Type any `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return KindUnknown, fmt.Errorf("parse %s schema: %w", typeName, err)
	}
	prop, ok := raw.Properties[fieldName]
	if !ok {
		return KindUnknown, nil
	}
	return classify(prop.Type), nil
}

// classify maps a JSON Schema "type" value to a FieldKind. The value may be a
// string ("array"), a slice of strings (["string","null"]), or absent (e.g.
// fields defined via `enum` or `const` — treated as scalar).
func classify(t any) Kind {
	switch v := t.(type) {
	case string:
		return kindFromString(v)
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				if k := kindFromString(s); k != KindUnknown {
					return k
				}
			}
		}
		return KindUnknown
	case nil:
		return KindScalar
	}
	return KindUnknown
}

func kindFromString(s string) Kind {
	switch s {
	case "array":
		return KindArray
	case "object":
		return KindObject
	case "string", "integer", "number", "boolean", "null":
		return KindScalar
	}
	return KindUnknown
}
