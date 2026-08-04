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
	writeFixtureMilestone(t, vault, "foo.m1-bar", "planned")

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
	writeFixtureTyped(t, vault, "30-decisions", "decision", "auth.0001-x")
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

// TestLink_RelationDependsOn writes a typed dependency edge into depends_on[]
// rather than related[], so `anvil list issue --ready` and Obsidian both see it.
func TestLink_RelationDependsOn(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "Dependent A")
	writeFixtureIssue(t, vault, "foo", "b", "Prereq B")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "issue", "foo.a", "issue", "foo.b", "--relation", "depends_on"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link --relation depends_on: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.a.md"))
	if err != nil {
		t.Fatal(err)
	}
	dep, _ := a.FrontMatter["depends_on"].([]any)
	if len(dep) != 1 || dep[0] != "[[issue.foo.b]]" {
		t.Errorf("depends_on = %v, want [[issue.foo.b]]", dep)
	}
	if _, ok := a.FrontMatter["related"]; ok {
		t.Errorf("related should be untouched, got %v", a.FrontMatter["related"])
	}
}

func TestLink_RelationDependsOn_RejectsUnknownRelation(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "a", "Dependent A")
	writeFixtureIssue(t, vault, "foo", "b", "Prereq B")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "issue", "foo.a", "issue", "foo.b", "--relation", "milestone"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error rejecting --relation milestone, got: %s", buf.String())
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

// writeFixtureTyped writes a minimal artifact of type typ into dir. Link-target
// resolution only needs the file to exist, so the frontmatter stays skeletal.
func writeFixtureTyped(t *testing.T, vault, dir, typ, id string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(vault, dir), 0o755); err != nil { //nolint:gosec // test fixture; 0755 matches vault convention
		t.Fatal(err)
	}
	a := &core.Artifact{
		Path: filepath.Join(vault, dir, id+".md"),
		FrontMatter: map[string]any{
			"type": typ, "title": id, "description": "fixture description",
			"created": "2026-01-01", "updated": "2026-01-01", "status": "active",
		},
		Body: "fixture body\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
}

// TestLink_CanonicalPrefixedTargetId pins the fix for the double-prefix trap:
// convention ids keep their type prefix on disk, and a system-design id given
// bare or type-prefixed both resolve to the same edge, so the id every anvil
// surface prints must produce the same edge as the bare form rather than a
// doubled, unresolvable one.
func TestLink_CanonicalPrefixedTargetId(t *testing.T) {
	cases := []struct {
		name       string
		tgtType    string
		tgtID      string
		wantTarget string
	}{
		{"convention canonical", "convention", "convention.sqlmesh", "[[convention.sqlmesh]]"},
		{"convention bare", "convention", "sqlmesh", "[[convention.sqlmesh]]"},
		{"system-design canonical", "system-design", "system-design.foo", "[[system-design.foo]]"},
		{"system-design bare", "system-design", "foo", "[[system-design.foo]]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := setupVault(t)
			writeFixturePlan(t, vault, "foo", "q2", "Q2")
			writeFixtureTyped(t, vault, "35-conventions", "convention", "convention.sqlmesh")
			writeFixtureDesign(t, vault, "foo", core.TypeSystemDesign, "Foo SD")

			cmd := newRootCmd()
			cmd.SetArgs([]string{"link", "plan", "foo.q2", tc.tgtType, tc.tgtID})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("link plan→%s %s: %v", tc.tgtType, tc.tgtID, err)
			}
			a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
			if err != nil {
				t.Fatal(err)
			}
			related, _ := a.FrontMatter["related"].([]any)
			if len(related) != 1 || related[0] != tc.wantTarget {
				t.Errorf("related = %v, want [%s]", related, tc.wantTarget)
			}
		})
	}
}

// TestLink_AlreadyTypedBareSlugTargetId pins the anvil.0234 regression: a
// target type that keys on a bare on-disk slug (learning, inbox, ...) must
// normalise an already-typed target id to the same edge a bare id produces,
// not silently concatenate a second prefix. anvil.0177 fixed this for types
// that keep their prefix on disk (convention, system-design); this is the
// same bug class recurring on the bare-slug side.
func TestLink_AlreadyTypedBareSlugTargetId(t *testing.T) {
	cases := []struct {
		name  string
		tgtID string
	}{
		{"already-typed", "learning.duckdb-arg-min-escalation"},
		{"doubled", "learning.learning.duckdb-arg-min-escalation"},
		{"bare", "duckdb-arg-min-escalation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := setupVault(t)
			writeFixturePlan(t, vault, "foo", "q2", "Q2")
			writeFixtureTyped(t, vault, "20-learnings", "learning", "duckdb-arg-min-escalation")

			cmd := newRootCmd()
			cmd.SetArgs([]string{"link", "plan", "foo.q2", "learning", tc.tgtID})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("link plan→learning %s: %v", tc.tgtID, err)
			}
			a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
			if err != nil {
				t.Fatal(err)
			}
			related, _ := a.FrontMatter["related"].([]any)
			want := "[[learning.duckdb-arg-min-escalation]]"
			if len(related) != 1 || related[0] != want {
				t.Errorf("related = %v, want [%s]", related, want)
			}
		})
	}
}

// TestLink_TargetLeadingSegmentEqualsTypeName pins the strip chain's
// every-rung probe: a legitimate canonical id whose leading segment equals the
// type name (a plan whose bare slug is `plan.q2`, on disk as plan.plan.q2.md)
// resolves only through the once-stripped candidate, which a fully-stripped-only
// probe would skip.
func TestLink_TargetLeadingSegmentEqualsTypeName(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")
	writeFixtureTyped(t, vault, "80-plans", "plan", "plan.plan.q2")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "plan", "plan.plan.q2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("link plan→plan plan.plan.q2: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
	if err != nil {
		t.Fatal(err)
	}
	related, _ := a.FrontMatter["related"].([]any)
	want := "[[plan.plan.q2]]"
	if len(related) != 1 || related[0] != want {
		t.Errorf("related = %v, want [%s]", related, want)
	}
}

// TestLink_RejectsMissingTarget pins the second half: a dead edge is refused at
// write time, and the error names both forms tried so the caller can tell a
// typo from a missing artifact.
func TestLink_RejectsMissingTarget(t *testing.T) {
	vault := setupVault(t)
	writeFixturePlan(t, vault, "foo", "q2", "Q2")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"link", "plan", "foo.q2", "convention", "convention.nope"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error linking to a missing convention")
	}
	for _, want := range []string{"[[convention.convention.nope]]", "[[convention.nope]]"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
	if err != nil {
		t.Fatal(err)
	}
	if related, ok := a.FrontMatter["related"]; ok {
		t.Errorf("related written despite refusal: %v", related)
	}
}

// TestLink_RejectsPlaceholderTarget pins the dead-edge hole the existence guard
// alone left open: `core.WikilinkTargetExists` reports false for an id carrying
// <, >, or whitespace (it can never name an artifact), but the link indexer's
// `^\[\[([^\]]+)\]\]$` still matches those tokens — so an unsubstituted
// `writing-issue` Phase 4b placeholder would land a real graph edge to nothing
// while `show --validate` stayed green.
func TestLink_RejectsPlaceholderTarget(t *testing.T) {
	cases := []struct {
		name    string
		tgtType string
		tgtID   string
	}{
		{"angle-bracket placeholder", "system-design", "<project>"},
		{"whitespace in id", "convention", "sqlmesh bad"},
		{"prefixed placeholder", "system-design", "system-design.<project>"},
		{"tab in id", "convention", "sqlmesh\tbad"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vault := setupVault(t)
			writeFixturePlan(t, vault, "foo", "q2", "Q2")

			cmd := newRootCmd()
			cmd.SetArgs([]string{"link", "plan", "foo.q2", tc.tgtType, tc.tgtID})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error linking to placeholder %q", tc.tgtID)
			}
			// The message quotes the id with %q, so a tab arrives escaped.
			if !strings.Contains(err.Error(), fmt.Sprintf("%q", tc.tgtID)) {
				t.Errorf("error %q does not name the offending target %q", err, tc.tgtID)
			}
			a, err := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.q2.md"))
			if err != nil {
				t.Fatal(err)
			}
			if related, ok := a.FrontMatter["related"]; ok {
				t.Errorf("related written despite refusal: %v", related)
			}
		})
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
