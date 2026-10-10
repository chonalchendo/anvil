package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/cli/errfmt"
	"github.com/chonalchendo/anvil/internal/core"
)

type createStatus string

const (
	statusCreated       createStatus = "created"
	statusAlreadyExists createStatus = "already_exists"
	statusUpdated       createStatus = "updated"
)

func emitCreateResult(cmd *cobra.Command, asJSON bool, id, path string, status createStatus, warnings []string, findings []*errfmt.ValidationError, changed []string, snap snapshotResult) error {
	if asJSON {
		payload := map[string]any{
			"id":     id,
			"path":   path,
			"status": string(status),
		}
		if status == statusUpdated {
			if changed == nil {
				changed = []string{}
			}
			payload["changed"] = changed
			if snap.SHA != "" {
				payload["snapshot"] = snap.SHA
			}
		}
		ws := jsonWarnings(warnings, findings)
		if snap.Warning != "" {
			ws = append(ws, map[string]string{"kind": "snapshot", "got": snap.Warning})
		}
		if len(ws) > 0 {
			payload["warnings"] = ws
		}
		out, _ := json.Marshal(payload)
		fmt.Fprintln(cmd.OutOrStdout(), string(out))
		return nil
	}
	if len(findings) > 0 {
		printValidationErrors(cmd, findings)
	}
	switch status {
	case statusCreated:
		fmt.Fprintln(cmd.OutOrStdout(), "created: "+path)
	case statusAlreadyExists:
		fmt.Fprintln(cmd.OutOrStdout(), "already_exists: "+path)
	case statusUpdated:
		fmt.Fprintln(cmd.OutOrStdout(), "updated: "+path)
		if snap.SHA != "" {
			fmt.Fprintln(cmd.OutOrStdout(), "snapshot: "+snap.SHA)
		}
		if snap.Warning != "" {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+snap.Warning)
		}
	}
	for _, w := range warnings {
		fmt.Fprintln(cmd.ErrOrStderr(), "warning: similar artifact exists: "+w+" (pass --force-new to skip)")
	}
	return nil
}

// jsonWarnings builds the success-envelope warnings array: near-duplicate
// hits first (their position predates validation findings, so .warnings[0]
// stays stable), then validation findings.
func jsonWarnings(similar []string, findings []*errfmt.ValidationError) []map[string]string {
	var ws []map[string]string
	for _, id := range similar {
		ws = append(ws, map[string]string{"kind": "similar", "id": id})
	}
	for _, f := range findings {
		ws = append(ws, map[string]string{"kind": "validation", "code": f.Code, "got": f.Got})
	}
	return ws
}

// createDrift returns the name of the first field that differs between
// fm/body and an existing artifact. Only flag-settable fields are
// compared; fields mutated via 'anvil set' (e.g. status) are ignored
// so retrying create after a status edit isn't drift.
func createDrift(t core.Type, fm, existing map[string]any, body, existingBody string) string {
	scalarFields := []string{"title", "description", "project"}
	switch t {
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
	if !sameBody(body, existingBody) {
		return "body"
	}
	return ""
}

// sameBody ignores surrounding whitespace: a saved body keeps a leading
// newline the flag value lacks, and an identical re-run must be a no-op.
func sameBody(a, b string) bool {
	return strings.TrimSpace(a) == strings.TrimSpace(b)
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
