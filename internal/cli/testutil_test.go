package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
)

// runCmd executes cmd with args, capturing stdout and stderr separately.
func runCmd(t *testing.T, cmd *cobra.Command, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

// listEnv mirrors the bounded list JSON envelope for tests.
type listEnv struct {
	Items     []listEnvItem `json:"items"`
	Total     int           `json:"total"`
	Returned  int           `json:"returned"`
	Truncated bool          `json:"truncated"`
}

type listEnvItem struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Created     string   `json:"created"`
	Project     string   `json:"project"`
	Path        string   `json:"path"`
	Tags        []string `json:"tags"`
}

func unmarshalListEnvelope(t *testing.T, s string) listEnv {
	t.Helper()
	var env listEnv
	if err := json.Unmarshal([]byte(s), &env); err != nil {
		t.Fatalf("invalid JSON envelope: %v\n%s", err, s)
	}
	return env
}

// newTestVaultWithIssues seeds n issues with descending created dates so
// the most recent sorts first. Returns vault root.
func newTestVaultWithIssues(t *testing.T, n int) string {
	t.Helper()
	vault := setupVault(t)
	for i := 0; i < n; i++ {
		// day stays in [1, 28] so we don't roll past month-end for reasonable n.
		day := ((n - i - 1) % 28) + 1
		date := fmt.Sprintf("2026-05-%02d", day)
		writeFixtureIssueDated(t, vault, "foo", fmt.Sprintf("i%03d", i), fmt.Sprintf("Issue %d", i), date)
	}
	return vault
}

// newTestVaultWithDatedIssues seeds one issue per supplied date.
func newTestVaultWithDatedIssues(t *testing.T, dates []string) string {
	t.Helper()
	vault := setupVault(t)
	for i, d := range dates {
		writeFixtureIssueDated(t, vault, "foo", fmt.Sprintf("i%03d", i), fmt.Sprintf("Issue %d", i), d)
	}
	return vault
}

// newTestVaultWithDatedInbox seeds one raw inbox artifact per supplied date.
func newTestVaultWithDatedInbox(t *testing.T, dates []string) string {
	t.Helper()
	vault := setupVault(t)
	for i, d := range dates {
		id := fmt.Sprintf("%s-foo-%d", d, i)
		path := filepath.Join(vault, "00-inbox", id+".md")
		a := &core.Artifact{
			Path: path,
			FrontMatter: map[string]any{
				"type": "inbox", "title": fmt.Sprintf("Inbox %d", i),
				"created": d, "status": "raw",
			},
			Body: "fixture body.\n",
		}
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
	return vault
}

// newTestVaultWithMixedInbox seeds three inbox artifacts across statuses:
// one raw, one promoted, one dropped.
func newTestVaultWithMixedInbox(t *testing.T) string {
	t.Helper()
	vault := setupVault(t)
	entries := []struct {
		id     string
		fields map[string]any
	}{
		{"2026-05-01-raw-0", map[string]any{
			"type": "inbox", "title": "raw entry",
			"created": "2026-05-01", "status": "raw",
		}},
		{"2026-05-02-promoted-1", map[string]any{
			"type": "inbox", "title": "promoted entry",
			"created": "2026-05-02", "status": "promoted",
			"promoted_to": "foo.x", "promoted_type": "issue",
		}},
		{"2026-05-03-dropped-2", map[string]any{
			"type": "inbox", "title": "dropped entry",
			"created": "2026-05-03", "status": "dropped",
		}},
	}
	for _, e := range entries {
		path := filepath.Join(vault, "00-inbox", e.id+".md")
		a := &core.Artifact{Path: path, FrontMatter: e.fields, Body: "fixture body.\n"}
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
	return vault
}

func writeFixtureIssueDated(t *testing.T, vault, project, slug, title, created string) string {
	t.Helper()
	id := project + "." + slug
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "issue", "title": title, "description": "fixture description",
			"created": created, "updated": created,
			"status": "open", "project": project, "severity": "medium",
			"tags": []any{"domain/dev-tools"},
		},
		Body: "## Context\n\nfixture body.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}
