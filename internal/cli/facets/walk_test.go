package facets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/cli/facets"
	"github.com/chonalchendo/anvil/internal/core"
)

func TestCollectValues_Empty(t *testing.T) {
	dir := t.TempDir()
	v := &core.Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	got, skipped, err := facets.CollectValues(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Errorf("CollectValues skipped unexpected paths: %v", skipped)
	}
	for _, facet := range []string{"domain", "activity", "pattern"} {
		if _, ok := got[facet]; !ok {
			t.Errorf("CollectValues missing facet bucket %q", facet)
		}
		if len(got[facet]) != 0 {
			t.Errorf("CollectValues[%q] non-empty for fresh vault: %v", facet, got[facet])
		}
	}
}

func TestCollectValues_AggregatesAcrossTypes(t *testing.T) {
	dir := t.TempDir()
	v := &core.Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	write := func(rel, fm string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("---\n"+fm+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("70-issues/foo.a.md", "type: issue\ntitle: a\ntags: [domain/dbt, activity/testing]")
	write("20-learnings/b.md", "type: learning\ntitle: b\ntags: [domain/postgres, pattern/idempotency]")
	write("60-threads/c.md", "type: thread\ntitle: c\ntags: [domain/dbt, activity/research]")
	write("00-inbox/d.md", "type: inbox\ntitle: d\ntags: [domain/should-be-included]")

	got, skipped, err := facets.CollectValues(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 0 {
		t.Errorf("CollectValues skipped unexpected paths: %v", skipped)
	}
	wantDomain := map[string]bool{"dbt": true, "postgres": true, "should-be-included": true}
	for v := range wantDomain {
		if _, ok := got["domain"][v]; !ok {
			t.Errorf("missing domain value %q", v)
		}
	}
	if _, ok := got["activity"]["testing"]; !ok {
		t.Error("missing activity/testing")
	}
	if _, ok := got["activity"]["research"]; !ok {
		t.Error("missing activity/research")
	}
	if _, ok := got["pattern"]["idempotency"]; !ok {
		t.Error("missing pattern/idempotency")
	}
}

func TestCollectValues_NonParseError_Propagates(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses file-permission checks")
	}
	dir := t.TempDir()
	v := &core.Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	// Write a valid artifact, then remove read permission so LoadArtifact hits
	// an OS-level error (not a frontmatter parse error).
	full := filepath.Join(dir, "70-issues", "unreadable.md")
	if err := os.WriteFile(full, []byte("---\ntype: issue\ntitle: t\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(full, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(full, 0o644) })

	_, _, err := facets.CollectValues(dir)
	if err == nil {
		t.Fatal("expected CollectValues to return an error for an unreadable file, got nil")
	}
}

func TestCollectValues_CorruptArtifact_SkippedNotError(t *testing.T) {
	dir := t.TempDir()
	v := &core.Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Valid artifact with a domain tag.
	write("70-issues/good.md", "---\ntype: issue\ntitle: good\ntags: [domain/dbt]\n---\n")
	// Corrupt frontmatter — backtick character cannot start any YAML token.
	write("70-issues/corrupt.md", "---\ntype: issue\ntitle: bad\ntags:\n  - `bad-backtick`\n---\n")

	got, skipped, err := facets.CollectValues(dir)
	if err != nil {
		t.Fatalf("CollectValues returned unexpected error: %v", err)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped path, got %d: %v", len(skipped), skipped)
	}
	if !strings.Contains(skipped[0], "corrupt.md") {
		t.Errorf("skipped path %q does not name the corrupt file", skipped[0])
	}
	// Valid artifact's tag must still be collected.
	if _, ok := got["domain"]["dbt"]; !ok {
		t.Error("missing domain/dbt from valid artifact")
	}
}
