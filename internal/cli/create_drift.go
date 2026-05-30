package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

type createStatus string

const (
	statusCreated       createStatus = "created"
	statusAlreadyExists createStatus = "already_exists"
	statusUpdated       createStatus = "updated"
)

func emitCreateResult(cmd *cobra.Command, asJSON bool, id, path string, status createStatus, warnings []string) error {
	if asJSON {
		payload := map[string]any{
			"id":     id,
			"path":   path,
			"status": string(status),
		}
		if len(warnings) > 0 {
			ws := make([]map[string]string, 0, len(warnings))
			for _, w := range warnings {
				ws = append(ws, map[string]string{"kind": "similar", "id": w})
			}
			payload["warnings"] = ws
		}
		out, _ := json.Marshal(payload)
		fmt.Fprintln(cmd.OutOrStdout(), string(out)) //nolint:errcheck // cobra writer methods ignore write errors by design
		return nil
	}
	switch status {
	case statusCreated:
		fmt.Fprintln(cmd.OutOrStdout(), "created: "+path) //nolint:errcheck // cobra writer methods ignore write errors by design
	case statusAlreadyExists:
		fmt.Fprintln(cmd.OutOrStdout(), "already_exists: "+path) //nolint:errcheck // cobra writer methods ignore write errors by design
	case statusUpdated:
		fmt.Fprintln(cmd.OutOrStdout(), "updated: "+path) //nolint:errcheck // cobra writer methods ignore write errors by design
	}
	for _, w := range warnings {
		fmt.Fprintln(cmd.ErrOrStderr(), "warning: similar artifact exists: "+w+" (pass --force-new to skip)") //nolint:errcheck // cobra writer methods ignore write errors by design
	}
	return nil
}

// createDrift returns the name of the first field that differs between
// fm/body and an existing artifact. Only flag-settable fields are
// compared; fields mutated via 'anvil set' (e.g. status) are ignored
// so retrying create after a status edit isn't drift.
func createDrift(t core.Type, fm, existing map[string]any, body, existingBody string) string {
	scalarFields := []string{"title", "description", "project"}
	switch t {
	case core.TypePlan:
		scalarFields = append(scalarFields, "issue")
	case core.TypeSweep:
		scalarFields = append(scalarFields, "scope", "breaking")
	case core.TypeInbox:
		scalarFields = append(scalarFields, "suggested_type", "suggested_project")
	}
	for _, f := range scalarFields {
		want := fm[f]
		got := existing[f]
		if want == nil && got == nil {
			continue
		}
		if want != got {
			return f
		}
	}
	if !tagsEqual(fm["tags"], existing["tags"]) {
		return "tags"
	}
	if strings.TrimRight(body, "\n\t ") != strings.TrimRight(existingBody, "\n\t ") {
		return "body"
	}
	return ""
}

func tagsEqual(a, b any) bool {
	as := tagSet(a)
	bs := tagSet(b)
	if len(as) != len(bs) {
		return false
	}
	for k := range as {
		if !bs[k] {
			return false
		}
	}
	return true
}

func tagSet(v any) map[string]bool {
	out := map[string]bool{}
	arr, _ := v.([]any)
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out[s] = true
		}
	}
	return out
}

func formatDriftError(_ *cobra.Command, id, field string, fm, existing map[string]any, body, existingBody string) error {
	var existingStr, newStr string
	switch field {
	case "tags":
		existingStr = renderTagArray(existing["tags"])
		newStr = renderTagArray(fm["tags"])
	case "body":
		existingStr = truncateBody(existingBody)
		newStr = truncateBody(body)
	default:
		existingStr = renderScalar(existing[field])
		newStr = renderScalar(fm[field])
	}
	return fmt.Errorf(
		"%w: %s already exists with different %s\n  existing: %s\n  new:      %s\n  retry with --update to overwrite, or use 'anvil set' to edit a single field",
		ErrCreateDrift, id, field, existingStr, newStr,
	)
}

func renderScalar(v any) string {
	if v == nil {
		return `""`
	}
	return fmt.Sprintf("%q", fmt.Sprintf("%v", v))
}

func renderTagArray(v any) string {
	if arr, ok := v.([]any); ok {
		ordered := make([]string, 0, len(arr))
		seen := map[string]bool{}
		for _, e := range arr {
			if s, ok := e.(string); ok && !seen[s] {
				ordered = append(ordered, s)
				seen[s] = true
			}
		}
		return "[" + strings.Join(ordered, ", ") + "]"
	}
	return "[]"
}

func truncateBody(s string) string {
	s = strings.TrimRight(s, "\n\t ")
	if len(s) <= 80 {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf("%q…", s[:80])
}
