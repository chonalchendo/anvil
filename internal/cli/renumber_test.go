package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

// writeDuplicateOrdinalPair lands alpha and beta on demo.0001, alpha linking
// beta — the synced-clone collision, with an inbound link to sweep.
func writeDuplicateOrdinalPair(t *testing.T, vault string) {
	t.Helper()
	for _, slug := range []string{"alpha", "beta"} {
		fm := map[string]any{
			"type": "issue", "title": slug, "description": "fixture description",
			"created": "2026-08-24", "updated": "2026-08-24",
			"status": "open", "project": "demo", "severity": "low",
			"tags": []any{"domain/dev-tools"}, "goal": slug + " is done",
		}
		if slug == "alpha" {
			fm["related"] = []any{"[[issue.demo.0001.beta]]"}
		}
		a := &core.Artifact{
			Path:        filepath.Join(vault, "70-issues", "issue.demo.0001."+slug+".md"),
			FrontMatter: fm,
			Body:        "## Context\n\nfixture body.\n",
		}
		if err := a.Save(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestShow_AmbiguousOrdinalRefusesNamingBoth(t *testing.T) {
	vault := setupVault(t)
	writeDuplicateOrdinalPair(t, vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "demo.0001", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("show demo.0001 succeeded on a duplicated ordinal:\n%s", out.String())
	}
	for _, want := range []string{"issue.demo.0001.alpha", "issue.demo.0001.beta"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err.Error(), want)
		}
	}
}

func TestRenumber_MovesToNextFreeOrdinalAndSweepsLinks(t *testing.T) {
	vault := setupVault(t)
	writeDuplicateOrdinalPair(t, vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"renumber", "issue", "issue.demo.0001.beta", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("renumber: %v\n%s", err, out.String())
	}
	var got renumberResult
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if got.ID != "issue.demo.0002.beta" || got.OldID != "issue.demo.0001.beta" {
		t.Errorf("ids = (%q ← %q), want issue.demo.0002.beta ← issue.demo.0001.beta", got.ID, got.OldID)
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "issue.demo.0001.beta.md")); err == nil {
		t.Error("old file still exists")
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "issue.demo.0002.beta.md")); err != nil {
		t.Errorf("new file missing: %v", err)
	}
	alpha, err := os.ReadFile(filepath.Join(vault, "70-issues", "issue.demo.0001.alpha.md")) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(alpha), "[[issue.demo.0002.beta]]") || strings.Contains(string(alpha), "[[issue.demo.0001.beta]]") {
		t.Errorf("inbound link not swept:\n%s", alpha)
	}
	if len(got.LinksRewritten) != 1 {
		t.Errorf("links_rewritten = %v, want the one alpha file", got.LinksRewritten)
	}

	// The shorthand is unambiguous again.
	cmd = newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "demo.0001", "--json"})
	out.Reset()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show demo.0001 after renumber: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), `"title":"alpha"`) {
		t.Errorf("show demo.0001 = %s, want alpha", out.String())
	}
}

func TestRenumber_ToOccupiedOrdinalRefuses(t *testing.T) {
	vault := setupVault(t)
	writeDuplicateOrdinalPair(t, vault)
	if err := os.WriteFile(filepath.Join(vault, "70-issues", "issue.demo.0002.gamma.md"), []byte("---\ntype: issue\n---\n"), 0o644); err != nil { //nolint:gosec // test file
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"renumber", "issue", "issue.demo.0001.beta", "--to", "2"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "taken") {
		t.Fatalf("err = %v, want taken-by refusal", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "issue.demo.0001.beta.md")); err != nil {
		t.Errorf("refused renumber moved the file: %v", err)
	}
}

func TestRenumber_NegativeToRefuses(t *testing.T) {
	vault := setupVault(t)
	writeDuplicateOrdinalPair(t, vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"renumber", "issue", "issue.demo.0001.beta", "--to", "-5"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("err = %v, want positive-ordinal refusal", err)
	}
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "issue.demo.0001.beta.md")); err != nil {
		t.Errorf("refused renumber moved the file: %v", err)
	}
}

func TestRenumber_ToOwnOrdinalIsNoop(t *testing.T) {
	vault := setupVault(t)
	writeDuplicateOrdinalPair(t, vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"renumber", "issue", "issue.demo.0001.beta", "--to", "1", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("retry onto own ordinal failed: %v", err)
	}
	if !strings.Contains(out.String(), `"id":"issue.demo.0001.beta"`) {
		t.Errorf("out = %s, want the unchanged id", out.String())
	}
}

func TestReindex_WarnsOnDuplicateOrdinal(t *testing.T) {
	vault := setupVault(t)
	writeDuplicateOrdinalPair(t, vault)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"reindex", "--json"})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if !strings.Contains(stderr.String(), "duplicate-ordinal") || !strings.Contains(stderr.String(), "demo.0001") {
		t.Errorf("stderr = %q, want a duplicate-ordinal WARN naming demo.0001", stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout.String())
	}
	dupes, _ := got["duplicate_ordinals"].([]any)
	if len(dupes) != 1 || dupes[0] != "issue.demo.0001" {
		t.Errorf("duplicate_ordinals = %v, want [issue.demo.0001]", got["duplicate_ordinals"])
	}
}
