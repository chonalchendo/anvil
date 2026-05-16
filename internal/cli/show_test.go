package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chonalchendo/anvil/internal/core"
)

func writeFixtureIssue(t *testing.T, vault, project, slug, title string) string {
	t.Helper()
	id := project + "." + slug
	path := filepath.Join(vault, "70-issues", id+".md")
	a := &core.Artifact{
		Path: path,
		FrontMatter: map[string]any{
			"type": "issue", "title": title, "description": "fixture description", "created": "2026-04-29",
			"updated": "2026-04-29", "status": "open", "project": project, "severity": "medium",
			"tags": []any{"domain/dev-tools"},
		},
		Body: "## Context\n\nfixture body.\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestShow_Text(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "bar", "Bar issue")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bar", "--body"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !bytes.Contains(out.Bytes(), []byte("Bar issue")) {
		t.Errorf("title missing from output:\n%s", got)
	}
	if !bytes.Contains(out.Bytes(), []byte("fixture body")) {
		t.Errorf("body missing from output:\n%s", got)
	}
}

func TestShow_JSON(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "bar", "Bar issue")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bar", "--body", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	// Frontmatter fields are flattened onto the top-level envelope so callers
	// read `status`, `title`, etc. with the same idiom as `anvil list --json`.
	if got["title"] != "Bar issue" {
		t.Errorf("title = %v", got["title"])
	}
	if _, present := got["frontmatter"]; present {
		t.Errorf("frontmatter key should not exist (flat shape); got: %v", got["frontmatter"])
	}
	if _, ok := got["body"].(string); !ok {
		t.Errorf("body missing or not string: %v", got["body"])
	}
	if _, ok := got["path"].(string); !ok {
		t.Errorf("path missing or not string: %v", got["path"])
	}
}

// TestShow_JSON_ShapeParityWithList pins the contract that motivated this
// change: a JSON-driven caller reads `status` (and other frontmatter fields)
// the same way from both verbs without verb-specific branching.
func TestShow_JSON_ShapeParityWithList(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "bar", "Bar issue")

	// show: frontmatter flattened onto top level.
	showOut, _, err := runCmd(t, newRootCmd(), "show", "issue", "foo.bar", "--json", "--no-body")
	if err != nil {
		t.Fatal(err)
	}
	var showGot map[string]any
	if err := json.Unmarshal([]byte(showOut), &showGot); err != nil {
		t.Fatalf("show JSON invalid: %v\n%s", err, showOut)
	}
	if showGot["status"] != "open" {
		t.Errorf("show: status=%v, want \"open\" at top level", showGot["status"])
	}
	if showGot["severity"] != "medium" {
		t.Errorf("show: severity=%v, want \"medium\" at top level", showGot["severity"])
	}

	// list: same key at the same path (items[0].status).
	listOut, _, err := runCmd(t, newRootCmd(), "list", "issue", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var listGot struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(listOut), &listGot); err != nil {
		t.Fatalf("list JSON invalid: %v\n%s", err, listOut)
	}
	if len(listGot.Items) == 0 {
		t.Fatal("list returned no items")
	}
	if listGot.Items[0]["status"] != "open" {
		t.Errorf("list: items[0].status=%v, want \"open\"", listGot.Items[0]["status"])
	}
}

// TestShow_JSON_EnvelopeKeysShadowFrontmatter pins collision behaviour: if an
// artifact's frontmatter has a key that collides with an envelope-reserved
// name (e.g. plan frontmatter carries its own `id`), the envelope value wins
// so callers can always rely on the top-level `id`/`path`/`body` semantics.
func TestShow_JSON_EnvelopeKeysShadowFrontmatter(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "70-issues", "foo.bar.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "d", "created": "2026-04-29",
			"status": "open", "project": "foo", "severity": "low",
			// Adversarial: frontmatter keys that collide with envelope fields.
			"id":   "frontmatter-id-should-lose",
			"path": "/frontmatter/path/should/lose",
		},
		Body: "body content",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	out, _, err := runCmd(t, newRootCmd(), "show", "issue", "foo.bar", "--json", "--no-body")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got["id"] != "foo.bar" {
		t.Errorf("id=%v, want \"foo.bar\" (envelope wins on collision)", got["id"])
	}
	if got["path"] == "/frontmatter/path/should/lose" {
		t.Errorf("path=%v, frontmatter value leaked through (envelope must win)", got["path"])
	}
}

// TestShow_IssueDefaultIncludesBody: bounded types default body=true so agents
// don't burn a round-trip on --body for every issue/inbox/decision/sweep view.
func TestShow_IssueDefaultIncludesBody(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "bar", "Bar issue")
	cmd := newRootCmd()
	out, _, _ := runCmd(t, cmd, "show", "issue", "foo.bar", "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	body, ok := got["body"].(string)
	if !ok {
		t.Fatalf("body missing or not string in default mode: %v", got["body"])
	}
	if !strings.Contains(body, "fixture body") {
		t.Errorf("body=%q, want to contain %q", body, "fixture body")
	}
	if got["status"] != "open" {
		t.Errorf("status=%v, want \"open\" (flattened)", got["status"])
	}
	if _, present := got["frontmatter"]; present {
		t.Error("frontmatter key should not exist (flat shape)")
	}
}

// TestShow_IssueNoBodyOptsOut: --no-body overrides the bounded-type default.
// JSON consumers can suppress body when frontmatter-only is what they want.
func TestShow_IssueNoBodyOptsOut(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "bar", "Bar issue")
	cmd := newRootCmd()
	out, _, _ := runCmd(t, cmd, "show", "issue", "foo.bar", "--no-body", "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["body"] != nil {
		t.Errorf("body=%v, want nil with --no-body", got["body"])
	}
}

// TestShow_PlanDefaultIsFrontmatterOnly: plan bodies can be large (tasks +
// waves), so plan keeps the frontmatter-only default. --body opts in.
func TestShow_PlanDefaultIsFrontmatterOnly(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "80-plans", "anv-1.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "plan", "title": "P", "description": "fixture description", "created": "2026-04-29",
			"status": "draft", "issue": "[[issue.foo.bar]]",
		},
		Body: "## Task: T1\nplan body content\n",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	out, _, _ := runCmd(t, cmd, "show", "plan", "anv-1", "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["body"] != nil {
		t.Errorf("plan body=%v, want nil (plan default is frontmatter-only)", got["body"])
	}
}

// TestShow_BodyNoBodyMutuallyExclusive: combining the two flags is a user
// error, not a silent precedence rule.
func TestShow_BodyNoBodyMutuallyExclusive(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "bar", "Bar issue")
	cmd := newRootCmd()
	_, _, err := runCmd(t, cmd, "show", "issue", "foo.bar", "--body", "--no-body")
	if err == nil {
		t.Fatal("expected error for --body + --no-body")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("err = %v, want mutually-exclusive message", err)
	}
}

func TestShow_BodyFlagPopulatesBody(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "70-issues", "foo.bar.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "d", "created": "2026-04-29",
			"status": "open", "project": "foo", "severity": "low",
		},
		Body: "body content",
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	out, _, _ := runCmd(t, cmd, "show", "issue", "foo.bar", "--body", "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got["body"] != "body content" {
		t.Errorf("body=%v, want \"body content\"", got["body"])
	}
}

func TestShow_BodyFlagClipsAt500Lines(t *testing.T) {
	vault := setupVault(t)
	body := strings.Repeat("line\n", 600)
	p := filepath.Join(vault, "70-issues", "foo.bar.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "d", "created": "2026-04-29",
			"status": "open", "project": "foo", "severity": "low",
		},
		Body: body,
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	out, errOut, _ := runCmd(t, cmd, "show", "issue", "foo.bar", "--body", "--json")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got["body_truncated"] != true {
		t.Error("body_truncated should be true")
	}
	if got["body_lines_total"].(float64) < 600 {
		t.Errorf("body_lines_total=%v want >=600", got["body_lines_total"])
	}
	if !strings.Contains(errOut, "500 of") {
		t.Errorf("expected clip hint on stderr, got %q", errOut)
	}
}

func TestShow_FullFlagRemoved(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "bar", "Bar issue")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bar", "--full"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected --full to be rejected as unknown flag")
	}
	if !strings.Contains(err.Error(), "unknown flag") && !strings.Contains(errOut.String(), "unknown flag") {
		t.Errorf("err should mention unknown flag, got: %v\nstderr: %s", err, errOut.String())
	}
}

func TestShow_MissingArtifact_ReturnsSentinel(t *testing.T) {
	setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "nonexistent"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Errorf("err = %v, want ErrArtifactNotFound", err)
	}
}

func TestShow_UnknownType_Errors(t *testing.T) {
	setupVault(t)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "bogus", "x"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestShowValidate_Issue_Clean(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "ok", "OK")
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.ok", "--validate"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected clean validate, got %v\n%s", err, out.String())
	}
}

func TestShowValidate_Issue_DanglingMilestone(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "70-issues", "foo.bad.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "fixture description", "created": "2026-04-29",
			"status": "open", "project": "foo", "severity": "low",
			"tags":      []any{"domain/dev-tools"},
			"milestone": "[[milestone.foo.ghost]]",
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bad", "--validate"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for dangling link")
	}
	if !errors.Is(err, ErrUnresolvedLinks) {
		t.Errorf("err = %v, want ErrUnresolvedLinks", err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("milestone.foo.ghost")) {
		t.Errorf("output missing target name:\n%s", stderr.String())
	}
}

func TestShowValidate_Milestone_DanglingArrayEntry(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "85-milestones", "foo.m.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "milestone", "title": "M", "description": "fixture description", "created": "2026-04-29",
			"status": "planned", "project": "foo",
			"kind":    "bucket",
			"related": []any{"[[issue.foo.ghost]]"},
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "milestone", "foo.m", "--validate"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if !errors.Is(err, ErrUnresolvedLinks) {
		t.Errorf("err = %v, want ErrUnresolvedLinks", err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("related[0]")) {
		t.Errorf("output missing field index:\n%s", stderr.String())
	}
}

func TestShowValidate_Issue_BadSchema(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "70-issues", "foo.bad.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "fixture description", "created": "2026-04-29",
			"status": "open", "project": "foo",
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bad", "--validate"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	err := cmd.Execute()
	if !errors.Is(err, ErrSchemaInvalid) {
		t.Errorf("err = %v, want ErrSchemaInvalid", err)
	}
}

func TestShowValidate_StdoutVsStderr(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "70-issues", "foo.bad.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "d", "created": "2026-04-29",
			"status": "open", "project": "foo", "severity": "low",
			"tags":      []any{"domain/dev-tools"},
			"milestone": "[[milestone.foo.ghost]]",
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bad", "--validate"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	_ = cmd.Execute() // err expected, ignore

	// Stdout should contain artifact view (frontmatter-only default).
	if !strings.Contains(out.String(), "title") {
		t.Errorf("stdout should contain artifact frontmatter (title), got:\n%s", out.String())
	}
	// Diagnostics should be on stderr, not stdout.
	if strings.Contains(out.String(), "schema:") || strings.Contains(out.String(), "links:") {
		t.Errorf("diagnostics leaked to stdout:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "links:") || !strings.Contains(errOut.String(), "milestone.foo.ghost") {
		t.Errorf("expected links diagnostic on stderr, got:\n%s", errOut.String())
	}
}

func TestShowValidate_JSON(t *testing.T) {
	vault := setupVault(t)
	p := filepath.Join(vault, "70-issues", "foo.bad.md")
	a := &core.Artifact{
		Path: p,
		FrontMatter: map[string]any{
			"type": "issue", "title": "x", "description": "fixture description", "created": "2026-04-29",
			"status": "open", "project": "foo", "severity": "low",
			"tags":      []any{"domain/dev-tools"},
			"milestone": "[[milestone.foo.ghost]]",
		},
	}
	if err := a.Save(); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	cmd.SetArgs([]string{"show", "issue", "foo.bad", "--validate", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	_ = cmd.Execute()

	// Assert wire-format keys are snake_case end-to-end: a struct with explicit
	// `json:"field"`/`json:"target"` tags would still accept CamelCase via
	// json.Unmarshal's case-insensitive matching, so we check the raw bytes.
	if !bytes.Contains(out.Bytes(), []byte(`"field":"milestone"`)) {
		t.Errorf("expected lowercase JSON key \"field\", got:\n%s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(`"target":"milestone.foo.ghost"`)) {
		t.Errorf("expected lowercase JSON key \"target\", got:\n%s", out.String())
	}

	var got struct {
		SchemaOK        bool `json:"schema_ok"`
		UnresolvedLinks []struct {
			Field  string `json:"field"`
			Target string `json:"target"`
		} `json:"unresolved_links"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if !got.SchemaOK {
		t.Errorf("schema_ok = false, want true")
	}
	if len(got.UnresolvedLinks) != 1 || got.UnresolvedLinks[0].Target != "milestone.foo.ghost" {
		t.Errorf("unresolved_links = %v", got.UnresolvedLinks)
	}
}

// TestShow_IncomingEdges asserts the acceptance criterion: create A and B,
// `anvil link A B`, `anvil show B` lists A under incoming — in both text and
// JSON outputs, grouped by source type with id+title.
func TestShow_IncomingEdges(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "Source issue", "--description", "src",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "Target issue", "--description", "tgt",
		"--tags", "domain/dev-tools")
	execCmd(t, "link", "issue", "demo.source-issue", "issue", "demo.target-issue")

	// JSON shape: incoming.<type> -> [{id, title}]
	out := execCmd(t, "show", "issue", "demo.target-issue", "--json", "--no-body")
	var got struct {
		Incoming map[string][]struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"incoming"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	issues, ok := got.Incoming["issue"]
	if !ok || len(issues) != 1 {
		t.Fatalf("incoming.issue = %v (want one entry), full: %v", issues, got.Incoming)
	}
	if issues[0].ID != "demo.source-issue" || issues[0].Title != "Source issue" {
		t.Errorf("incoming entry = %+v, want {demo.source-issue, Source issue}", issues[0])
	}

	// Text output: section header + grouped entry.
	text := execCmd(t, "show", "issue", "demo.target-issue", "--no-body")
	if !strings.Contains(text, "Incoming links:") {
		t.Errorf("text output missing section header:\n%s", text)
	}
	if !strings.Contains(text, "demo.source-issue") || !strings.Contains(text, "Source issue") {
		t.Errorf("text output missing incoming entry id+title:\n%s", text)
	}
}

// TestShow_NoIncomingFlagSuppresses asserts --no-incoming removes the section
// (text) and drops the JSON key entirely (`omitempty`).
func TestShow_NoIncomingFlagSuppresses(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "A", "--description", "a",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "B", "--description", "b",
		"--tags", "domain/dev-tools")
	execCmd(t, "link", "issue", "demo.a", "issue", "demo.b")

	out := execCmd(t, "show", "issue", "demo.b", "--no-incoming", "--json", "--no-body")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if _, present := got["incoming"]; present {
		t.Errorf("incoming key should be absent under --no-incoming, got: %v", got["incoming"])
	}

	text := execCmd(t, "show", "issue", "demo.b", "--no-incoming", "--no-body")
	if strings.Contains(text, "Incoming links:") {
		t.Errorf("text output should not contain section under --no-incoming:\n%s", text)
	}
}

// TestShow_PrefixedIDResolvesLikeBareID asserts parity with transition/set:
// "issue.foo.bar" and "foo.bar" must resolve the same artifact.
func TestShow_PrefixedIDResolvesLikeBareID(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "foo", "bar", "Bar issue")

	cases := []struct {
		name string
		id   string
	}{
		{"bare", "foo.bar"},
		{"prefixed", "issue.foo.bar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newRootCmd()
			out, _, err := runCmd(t, cmd, "show", "issue", tc.id, "--json", "--no-body")
			if err != nil {
				t.Fatalf("id=%q: unexpected error: %v", tc.id, err)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, out)
			}
			if got["title"] != "Bar issue" {
				t.Errorf("id=%q: title=%v, want \"Bar issue\"", tc.id, got["title"])
			}
		})
	}
}

// TestShow_BareProjectMatchesType guards against stripTypePrefix mis-resolving
// a bare ID whose project name equals the artifact type. project="issue",
// slug="foo" → bare id "issue.foo"; it must NOT be stripped to "foo".
func TestShow_BareProjectMatchesType(t *testing.T) {
	vault := setupVault(t)
	writeFixtureIssue(t, vault, "issue", "foo", "Issue-project issue")

	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "show", "issue", "issue.foo", "--json", "--no-body")
	if err != nil {
		t.Fatalf("id=%q: unexpected error: %v", "issue.foo", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got["title"] != "Issue-project issue" {
		t.Errorf("title=%v, want \"Issue-project issue\"", got["title"])
	}
}

// TestShow_NoIncomingEdgesRendersCleanly ensures the section header doesn't
// dangle when no incoming edges exist.
func TestShow_NoIncomingEdgesRendersCleanly(t *testing.T) {
	vault := t.TempDir()
	t.Setenv("ANVIL_VAULT", vault)
	execCmd(t, "init", vault)
	execCmd(t, "create", "issue",
		"--project", "demo", "--title", "Lonely", "--description", "lonely",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain")

	text := execCmd(t, "show", "issue", "demo.lonely", "--no-body")
	if strings.Contains(text, "Incoming links:") {
		t.Errorf("section header should be absent when no incoming edges:\n%s", text)
	}

	out := execCmd(t, "show", "issue", "demo.lonely", "--json", "--no-body")
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if _, present := got["incoming"]; present {
		t.Errorf("incoming key should be omitted when empty, got: %v", got["incoming"])
	}
}

// TestShow_Skill prints the bundled SKILL.md body verbatim. Skills are not
// vault artifacts but the show verb resolves them from the embedded bundle so
// agents stop bouncing to grep on every smoke test.
func TestShow_Skill(t *testing.T) {
	cmd := newRootCmd()
	out, _, err := runCmd(t, cmd, "show", "skill", "capturing-inbox")
	if err != nil {
		t.Fatalf("show skill capturing-inbox: %v", err)
	}
	if !strings.Contains(out, "name: capturing-inbox") {
		t.Errorf("expected SKILL.md frontmatter (name: capturing-inbox), got:\n%s", out)
	}
}

// TestShow_SkillUnknown surfaces the available skill list so agents can
// self-correct from a typo without needing to grep the repo.
func TestShow_SkillUnknown(t *testing.T) {
	cmd := newRootCmd()
	_, _, err := runCmd(t, cmd, "show", "skill", "no-such-skill")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unknown skill") {
		t.Errorf("error should name the failure mode, got: %s", msg)
	}
	if !strings.Contains(msg, "capturing-inbox") {
		t.Errorf("error should list available skills, got: %s", msg)
	}
}
