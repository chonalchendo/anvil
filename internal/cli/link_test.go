package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func writeFixtureContract(t *testing.T, vault, project, slug string) string {
	t.Helper()
	id := project + "." + slug
	dir := filepath.Join(vault, "75-contracts")
	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec // test fixture; 0755 matches vault convention
		t.Fatal(err)
	}
	path := filepath.Join(dir, id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "contract", "title": "Data boundaries",
			"description": "what the pipeline does / does not",
			"created":     "2026-06-01", "updated": "2026-06-01",
			"status": "draft", "project": project, "kind": "data",
			"tags": []any{},
		},
		Body: "## Boundaries\n\ndoes: x\ndoes not: y\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFixturePlan(t *testing.T, vault, project, slug, title string) string {
	t.Helper()
	path := filepath.Join(vault, "80-plans", project+"."+slug+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "plan", "id": project + "-" + slug, "slug": slug, "title": title,
			"description": "fixture description",
			"created":     "2026-04-29", "updated": "2026-04-29", "status": "draft",
			"plan_version": 1, "project": project,
			"issue": "[[issue." + project + "." + slug + "]]",
			"tasks": []any{map[string]any{
				"id": "T1", "title": "x", "kind": "tdd",
				"files": []any{}, "depends_on": []any{}, "verify": "true",
			}},
		},
		Body: "## Task: T1\n\nfixture task body.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLink_PlanToMilestone(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "milestone", "foo.m1-bar"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
	if err != nil {
		t.Fatal(err)
	}
	related, _ := a.FrontMatter["related"].([]any)
	if len(related) != 1 || related[0] != "[[milestone.foo.m1-bar]]" {
		t.Errorf("related = %v", related)
	}
}

func TestLink_ExternalAppendsURI(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "--external", "https://github.com/chonalchendo/anvil/pull/13"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
	if err != nil {
		t.Fatal(err)
	}
	ext, _ := a.FrontMatter["external_links"].([]any)
	if len(ext) != 1 || ext[0] != "https://github.com/chonalchendo/anvil/pull/13" {
		t.Fatalf("external_links = %v", ext)
	}
}

func TestLink_ExternalIdempotent(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")
	for i := 0; i < 2; i++ {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"link", "plan", "foo.q2", "--external", "abc1234"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
	if err != nil {
		t.Fatal(err)
	}
	ext, _ := a.FrontMatter["external_links"].([]any)
	if len(ext) != 1 {
		t.Fatalf("external_links len = %d, want 1 (idempotent): %v", len(ext), ext)
	}
}

func TestLink_ExternalRejectsTargetArgs(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "issue", "foo.x", "--external", "https://x"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error, got: %s", buf.String())
	}
}

func TestLink_ExternalRejectsReadMode(t *testing.T) {
	_ = setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "--from", "demo.a", "--external", "https://x"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestLink_ExternalRejectsWhitespaceOnly(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "--external", "   "})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error rejecting whitespace-only --external, got: %s", buf.String())
	}
}

func TestLink_AnyPair_WritesToRelated(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "decision", "auth.0001-x"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
	if err != nil {
		t.Fatal(err)
	}
	related, _ := a.FrontMatter["related"].([]any)
	if len(related) != 1 || related[0] != "[[decision.auth.0001-x]]" {
		t.Errorf("related = %v", related)
	}
}

// TestLink_IssueToContract confirms Option-A contract routing: an issue can
// link to its governing contract and the wikilink lands in related[].
func TestLink_IssueToContract(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "i001", "Add dedup")
	writeFixtureContract(t, vault, "foo", "data-bounds")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "issue", "foo.i001", "contract", "foo.data-bounds"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link issue→contract: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.i001.md"))
	if err != nil {
		t.Fatal(err)
	}
	related, _ := a.FrontMatter["related"].([]any)
	if len(related) != 1 || related[0] != "[[contract.foo.data-bounds]]" {
		t.Errorf("related = %v, want [[contract.foo.data-bounds]]", related)
	}
}

// TestShow_IssueJSON_ExposesContractLink confirms that show issue --json
// surfaces the contract wikilink so a worker can discover and follow it.
func TestShow_IssueJSON_ExposesContractLink(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "i001", "Add dedup")
	writeFixtureContract(t, vault, "foo", "data-bounds")

	// Link then show.
	if _, err := runArgs(t, "link", "issue", "foo.i001", "contract", "foo.data-bounds"); err != nil {
		t.Fatalf("link: %v", err)
	}
	out, err := runArgs(t, "show", "issue", "foo.i001", "--json")
	if err != nil {
		t.Fatalf("show issue --json: %v\n%s", err, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	// The contract wikilink must appear somewhere in the JSON output so a
	// worker can discover and load the governing contract.
	raw, _ := json.Marshal(got)
	if !strings.Contains(string(raw), "contract.foo.data-bounds") {
		t.Errorf("contract link not found in show issue --json output:\n%s", string(raw))
	}
}

// writeNumberedFixtureIssue writes a numbered-format issue (<project>.NNNN.<slug>.md)
// so link tests can exercise short-id resolution with a realistic vault layout.
func writeNumberedFixtureIssue(t *testing.T, vault, project string, ordinal int, slug, title string) (id, path string) {
	t.Helper()
	id = fmt.Sprintf("%s.%04d.%s", project, ordinal, slug)
	path = filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "issue", "title": title, "description": "fixture description",
			"created": "2026-06-12", "updated": "2026-06-12",
			"status": "open", "project": project, "severity": "medium",
			"tags": []any{"domain/cli"}, "goal": "fixture goal is done",
		},
		Body: fixtureIssueBody,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return id, path
}

// TestLink_ShortIdResolution_IssueToContract confirms that a project-qualified
// short numeric id (e.g. "foo.0042") resolves to the full slug id and lands
// the wikilink in related[] — matching the behaviour of show/set/transition.
func TestLink_ShortIdResolution_IssueToContract(t *testing.T) {
	vault := setupVault(t)
	id, issuePath := writeNumberedFixtureIssue(t, vault, "foo", 42, "add-dedup", "Add dedup")
	writeFixtureContract(t, vault, "foo", "data-bounds")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "issue", "foo.0042", "contract", "foo.data-bounds"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link with short id foo.0042: %v", err)
	}
	a, err := core.LoadArtifact(issuePath)
	if err != nil {
		t.Fatal(err)
	}
	related, _ := a.FrontMatter["related"].([]any)
	if len(related) != 1 || related[0] != "[[contract.foo.data-bounds]]" {
		t.Errorf("related = %v, want [[contract.foo.data-bounds]] (full id = %s)", related, id)
	}
}

// TestLink_ShortIdResolution_ExternalLink confirms that --external also
// resolves the short numeric source id.
func TestLink_ShortIdResolution_ExternalLink(t *testing.T) {
	vault := setupVault(t)
	_, issuePath := writeNumberedFixtureIssue(t, vault, "foo", 7, "add-dedup", "Add dedup")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "issue", "foo.0007", "--external", "https://github.com/x/y/pull/1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link --external with short id foo.0007: %v", err)
	}
	a, err := core.LoadArtifact(issuePath)
	if err != nil {
		t.Fatal(err)
	}
	ext, _ := a.FrontMatter["external_links"].([]any)
	if len(ext) != 1 || ext[0] != "https://github.com/x/y/pull/1" {
		t.Errorf("external_links = %v, want [https://github.com/x/y/pull/1]", ext)
	}
}

// TestLink_ShortIdResolution_NonZeroOnMiss confirms that a short id that
// matches no file produces a non-zero exit — never a silent no-op.
func TestLink_ShortIdResolution_NonZeroOnMiss(t *testing.T) {
	vault := setupVault(t)
	writeFixtureContract(t, vault, "foo", "data-bounds")
	// No issue with ordinal 9999 exists.
	cmd := newRootCmd()
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"link", "issue", "foo.9999", "contract", "foo.data-bounds"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected non-zero exit for missing short id foo.9999 in vault %s", vault)
	}
}

// TestShow_Contract_Body pins the load leg of the discover-then-load path: once
// a worker follows the wikilink, `show contract <id> --body` must surface the
// contract's boundary prose.
func TestShow_Contract_Body(t *testing.T) {
	vault := setupVault(t)
	writeFixtureContract(t, vault, "foo", "data-bounds")

	out, err := runArgs(t, "show", "contract", "foo.data-bounds", "--body")
	if err != nil {
		t.Fatalf("show contract --body: %v\n%s", err, out)
	}
	if !strings.Contains(out, "does: x") {
		t.Errorf("contract body not surfaced in show contract --body output:\n%s", out)
	}
}
