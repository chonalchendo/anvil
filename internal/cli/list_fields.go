package cli

import (
	"encoding/json"
	"io"
	"slices"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/cli/output"
)

// listItemFields is the exhaustive set of JSON keys a listItem can carry.
// Used to reject unknown --fields values before any output is written.
var listItemFields = []string{
	"id", "type", "title", "description", "status",
	"severity", "created", "project", "milestone", "tags",
	"path", "missing_section",
}

// parseFields splits the comma-separated --fields flag and rejects any value
// not in listItemFields. An unknown field returns a bad_flag_value error so
// callers detect typos via a non-zero exit code regardless of --json mode.
func parseFields(flag string) ([]string, error) {
	if flag == "" {
		return nil, nil
	}
	fields := splitTags([]string{flag}) // reuse comma-split logic
	for _, f := range fields {
		if !slices.Contains(listItemFields, f) {
			return nil, errfmt.NewStructured("bad_flag_value").
				Set("flag", "fields").
				Set("value", f).
				Set("allowed", listItemFields)
		}
	}
	return fields, nil
}

// writeProjectedListJSON emits the canonical bounded-list envelope with each
// item reduced to only the requested field keys, routing through
// output.WriteListJSON so the projected and full --json paths serialise
// identically (same envelope shape, same HTML escaping).
func writeProjectedListJSON(w io.Writer, items []listItem, total, returned int, fields []string) error {
	projected := make([]map[string]any, 0, len(items))
	for _, item := range items {
		// Round-trip through JSON to obtain a key→value map without
		// maintaining a hand-coded field registry.
		b, err := json.Marshal(item)
		if err != nil {
			return err
		}
		var full map[string]any
		if err := json.Unmarshal(b, &full); err != nil {
			return err
		}
		row := make(map[string]any, len(fields))
		for _, f := range fields {
			if v, ok := full[f]; ok {
				row[f] = v
			} else {
				// Field exists in schema but was omitted (omitempty zero
				// value); include it as null so callers see the key.
				row[f] = nil
			}
		}
		projected = append(projected, row)
	}
	return output.WriteListJSON(w, projected, total, returned)
}
