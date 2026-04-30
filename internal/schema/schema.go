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
