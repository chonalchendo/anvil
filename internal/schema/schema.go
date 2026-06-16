// Package schema validates vault frontmatter against embedded JSON Schemas.
package schema

import (
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var compiler = func() *jsonschema.Compiler {
	c := jsonschema.NewCompiler()
	entries, err := EmbeddedFS.ReadDir(".")
	if err != nil {
		panic(fmt.Sprintf("read embedded schemas dir: %v", err))
	}
	for _, e := range entries {
		b, err := EmbeddedFS.ReadFile(e.Name())
		if err != nil {
			panic(fmt.Sprintf("read embedded schema %s: %v", e.Name(), err))
		}
		var raw any
		if err := json.Unmarshal(b, &raw); err != nil {
			panic(fmt.Sprintf("parse %s: %v", e.Name(), err))
		}
		if err := c.AddResource("https://anvil.dev/schemas/"+e.Name(), raw); err != nil {
			panic(err)
		}
	}
	return c
}()

// Validate runs frontmatter against the schema for typeName (e.g. "issue", "plan").
func Validate(typeName string, fm map[string]any) error {
	sch, err := compiler.Compile("https://anvil.dev/schemas/" + typeName + ".schema.json")
	if err != nil {
		return fmt.Errorf("compile %s schema: %w", typeName, err)
	}
	return sch.Validate(fm)
}

// ValidateField validates only the value of a single field against its property
// schema. Required-field checks for other fields are intentionally skipped so
// a one-field edit on a legacy artifact that is missing an unrelated required
// field does not fail. If fieldName is not declared in the schema, no error is
// returned (unknown / ad-hoc fields are accepted).
func ValidateField(typeName, fieldName string, value any) error {
	b, err := EmbeddedFS.ReadFile(typeName + ".schema.json")
	if err != nil {
		return fmt.Errorf("read %s schema: %w", typeName, err)
	}
	var raw struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("parse %s schema: %w", typeName, err)
	}
	propSchema, ok := raw.Properties[fieldName]
	if !ok {
		// Ad-hoc field; nothing to validate.
		return nil
	}
	// Build a minimal wrapper schema that validates only this property, with no
	// required array, so unrelated missing fields are never flagged.
	wrapper, err := json.Marshal(map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"properties":           map[string]json.RawMessage{fieldName: propSchema},
		"additionalProperties": true,
	})
	if err != nil {
		return fmt.Errorf("build field schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	var wrapperRaw any
	if err := json.Unmarshal(wrapper, &wrapperRaw); err != nil {
		return fmt.Errorf("parse field schema: %w", err)
	}
	url := "https://anvil.dev/schemas/_field_" + typeName + "_" + fieldName + ".json"
	if err := c.AddResource(url, wrapperRaw); err != nil {
		return fmt.Errorf("register field schema: %w", err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		return fmt.Errorf("compile field schema: %w", err)
	}
	return sch.Validate(map[string]any{fieldName: value})
}
