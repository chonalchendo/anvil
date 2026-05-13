package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/chonalchendo/anvil/internal/core"
	"github.com/chonalchendo/anvil/internal/schema"
)

func setupVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ANVIL_VAULT", dir)
	v := &core.Vault{Root: dir}
	if err := v.Scaffold(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCreate_Issue_WritesValidFile(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue", "--title", "Fix login bug", "--description", "test description", "--tags", "domain/dev-tools", "--allow-new-facet=domain"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create issue: %v\nstdout: %s", err, out.String())
	}

	path := filepath.Join(vault, "70-issues", "foo.fix-login-bug.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s", path)
	}
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["title"] != "Fix login bug" {
		t.Errorf("title = %v", a.FrontMatter["title"])
	}
	if err := schema.Validate("issue", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails schema: %v", err)
	}
}

func TestCreateMilestone_NoOrdinal(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "milestone", "--title", "CLI substrate", "--description", "test description"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	want := filepath.Join(vault, "85-milestones", "foo.cli-substrate.md")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected %s: %v", want, err)
	}
}

func TestCreate_Inbox_NoProjectNeeded(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir()) // not a git repo

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "inbox", "--title", "Streaming feels laggy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create inbox: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if err != nil || len(entries) != 1 {
		t.Errorf("expected 1 file in 00-inbox, got %d (%v)", len(entries), err)
	}
	if !strings.HasSuffix(entries[0].Name(), "-streaming-feels-laggy.md") {
		t.Errorf("got %s", entries[0].Name())
	}
}

func TestCreate_JSON_ReturnsIDAndPath(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue", "--title", "x", "--description", "test description", "--json", "--tags", "domain/dev-tools", "--allow-new-facet=domain"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got["id"] != "foo.x" {
		t.Errorf("id = %q", got["id"])
	}
	if !strings.HasPrefix(got["path"], vault) {
		t.Errorf("path = %q, expected under %q", got["path"], vault)
	}
}

func TestCreate_Decision_TopicScoped(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "decision", "--title", "use jwt", "--topic", "auth", "--description", "test description", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "30-decisions", "auth.0001-use-jwt.md")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("missing %s", path)
	}
}

func TestCreatePlan_NewSchema_Succeeds(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "plan",
		"--title", "Streaming token counter",
		"--description", "test description",
		"--issue", "[[issue.foo.streaming]]",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create: %v\n%s", err, out.String())
	}
	// Fresh plan parses cleanly and passes schema; ValidatePlan is gated to
	// the draft→locked transition (the placeholder T1 ships verify: "true",
	// which the validator rejects as a no-op — by design).
	//
	// Plan slug defaults to the linked issue's slug (`streaming`), not the
	// plan title — keeps linked-artifact pairs anchored on the same slug.
	path := filepath.Join(vault, "80-plans", "foo.streaming.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}
	if _, err := core.LoadPlan(path); err != nil {
		t.Fatalf("load plan: %v", err)
	}
}

func TestCreate_Thread_WritesValidFile(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir()) // not a git repo — thread needs no project

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "thread", "--title", "Research ducklake", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create thread: %v\nstdout: %s", err, out.String())
	}

	path := filepath.Join(vault, "60-threads", "research-ducklake.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s", path)
	}
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["type"] != "thread" {
		t.Errorf("type = %v", a.FrontMatter["type"])
	}
	if a.FrontMatter["status"] != "open" {
		t.Errorf("status = %v", a.FrontMatter["status"])
	}
	if a.FrontMatter["diataxis"] != "explanation" {
		t.Errorf("diataxis = %v", a.FrontMatter["diataxis"])
	}
	if err := schema.Validate("thread", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails schema: %v", err)
	}
}

func TestCreate_Learning_WritesValidFile(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "learning", "--title", "Postgres FK locks block writes", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create learning: %v\nstdout: %s", err, out.String())
	}

	path := filepath.Join(vault, "20-learnings", "postgres-fk-locks-block-writes.md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatalf("loading learning: %v", err)
	}
	if a.FrontMatter["type"] != "learning" {
		t.Errorf("type = %v, want learning", a.FrontMatter["type"])
	}
	if a.FrontMatter["status"] != "draft" {
		t.Errorf("status = %v, want draft", a.FrontMatter["status"])
	}
	if a.FrontMatter["confidence"] != "low" {
		t.Errorf("confidence = %v, want low", a.FrontMatter["confidence"])
	}
	if a.FrontMatter["diataxis"] != "explanation" {
		t.Errorf("diataxis = %v, want explanation", a.FrontMatter["diataxis"])
	}
	if err := schema.Validate("learning", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails schema: %v", err)
	}
}

func TestCreate_Issue_WithBody_FlagRoundTrips(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue", "--title", "x", "--description", "test description", "--body", "## Context\n\nFrom flag.", "--tags", "domain/dev-tools", "--allow-new-facet=domain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.x.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.Body, "From flag.") {
		t.Errorf("body = %q", a.Body)
	}
}

func TestCreate_Issue_EmptyBody_Unchanged(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue", "--title", "x", "--description", "test description", "--tags", "domain/dev-tools", "--allow-new-facet=domain"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "70-issues", "foo.x.md"))
	if strings.TrimSpace(a.Body) != "" {
		t.Errorf("expected empty body, got %q", a.Body)
	}
}

func TestCreatePlan_BodyReplacesT1Seed_ValidWhenWellFormed(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	body := "\n## Task: T1\n\n" + strings.Repeat("Author-supplied T1 description that exceeds the 200-char body floor. ", 4) + "\n"

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "plan",
		"--title", "Author body",
		"--description", "test description",
		"--issue", "[[issue.foo.author-body-target]]",
		"--body", body,
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create plan: %v\n%s", err, stderr.String())
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.author-body-target.md"))
	if !strings.Contains(a.Body, "Author-supplied T1") {
		t.Errorf("body did not replace T1 seed: %q", a.Body)
	}
}

func TestCreatePlan_RequiresIssue(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "plan", "--title", "x"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error: missing --issue")
	}
}

func TestCreateMilestone_SeedsAcceptanceSlot(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "milestone", "--title", "CLI substrate", "--description", "test description"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "85-milestones", "foo.cli-substrate.md"))
	if err != nil {
		t.Fatal(err)
	}
	acc, ok := a.FrontMatter["acceptance"].([]any)
	if !ok {
		t.Fatalf("acceptance field missing or wrong type: %#v", a.FrontMatter["acceptance"])
	}
	if len(acc) != 0 {
		t.Errorf("acceptance = %v, want empty slice", acc)
	}
	if err := schema.Validate("milestone", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails milestone schema: %v", err)
	}
}

func TestCreate_ProductDesign_WritesValidFile(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "product-design", "--title", "Anvil product design", "--description", "test description"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create: %v\n%s", err, out.String())
	}

	path := filepath.Join(vault, "05-projects", "foo", "product-design.md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}
	if a.FrontMatter["type"] != "product-design" {
		t.Errorf("type = %v", a.FrontMatter["type"])
	}
	if a.FrontMatter["project"] != "foo" {
		t.Errorf("project = %v", a.FrontMatter["project"])
	}
	if strings.TrimSpace(a.Body) != "" {
		t.Errorf("expected empty body, got %q", a.Body)
	}
	if err := schema.Validate("product-design", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails schema: %v", err)
	}
}

func TestCreate_ProductDesign_Idempotent(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	args := []string{"create", "product-design",
		"--title", "Foo product",
		"--description", "summary line",
		"--json",
	}

	cmd1 := newRootCmd()
	cmd1.SetArgs(args)
	var out1 bytes.Buffer
	cmd1.SetOut(&out1)
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first create: %v", err)
	}

	path := filepath.Join(vault, "05-projects", "foo", "product-design.md")
	statBefore, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	cmd2 := newRootCmd()
	cmd2.SetArgs(args)
	var out2 bytes.Buffer
	cmd2.SetOut(&out2)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second create: %v", err)
	}
	var second map[string]string
	if err := json.Unmarshal(out2.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if second["status"] != "already_exists" {
		t.Errorf("status = %q, want already_exists", second["status"])
	}
	if second["id"] != "product-design" {
		t.Errorf("id = %q, want product-design (singleton convention)", second["id"])
	}

	statAfter, _ := os.Stat(path)
	if !statBefore.ModTime().Equal(statAfter.ModTime()) {
		t.Errorf("mtime changed on idempotent singleton re-run")
	}
}

func TestCreate_ProductDesign_RequiresProject(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir()) // not a git repo

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "product-design", "--title", "X"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error: requires project")
	}
}

func TestCreate_Sweep_BreakingTrue(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir()) // not a git repo — sweep is exempt

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "sweep", "--title", "CLI rename", "--description", "test description", "--scope", "cli", "--breaking"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create sweep: %v\n%s", err, out.String())
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "50-sweeps"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in 50-sweeps, got %d", len(entries))
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "50-sweeps", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["breaking"] != true {
		t.Errorf("breaking = %v, want true", a.FrontMatter["breaking"])
	}
	if a.FrontMatter["scope"] != "cli" {
		t.Errorf("scope = %v", a.FrontMatter["scope"])
	}
	if err := schema.Validate("sweep", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails schema: %v", err)
	}
}

func TestCreate_Sweep_BreakingFalseExplicit(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "sweep", "--title", "Docs polish", "--description", "test description", "--scope", "docs", "--breaking=false"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create sweep: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "50-sweeps"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "50-sweeps", entries[0].Name()))
	if a.FrontMatter["breaking"] != false {
		t.Errorf("breaking = %v, want false", a.FrontMatter["breaking"])
	}
	if err := schema.Validate("sweep", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails schema: %v", err)
	}
}

func TestCreate_Sweep_MissingScope(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "sweep", "--title", "X", "--breaking"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error: missing --scope")
	} else if !strings.Contains(err.Error(), "scope") {
		t.Errorf("error = %v, want mention of scope", err)
	}
}

func TestCreate_Sweep_MissingBreaking(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "sweep", "--title", "X", "--scope", "cli"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error: --breaking must be set explicitly")
	} else if !strings.Contains(err.Error(), "breaking") {
		t.Errorf("error = %v, want mention of breaking", err)
	}
}

func TestCreate_SystemDesign_WritesValidFile(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "system-design", "--title", "Anvil system design", "--description", "test description"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create: %v", err)
	}
	path := filepath.Join(vault, "05-projects", "foo", "system-design.md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}
	if a.FrontMatter["type"] != "system-design" {
		t.Errorf("type = %v", a.FrontMatter["type"])
	}
	if _, present := a.FrontMatter["product_design"]; present {
		t.Errorf("product_design should not be seeded; got %v", a.FrontMatter["product_design"])
	}
	if err := schema.Validate("system-design", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails schema: %v", err)
	}
}

const fakeSessionUUID = "01234567-89ab-cdef-0123-456789abcdef"

func TestCreateSession_WritesValidFile(t *testing.T) {
	vault := setupVault(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "session", "--session-id", fakeSessionUUID})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create session: %v\n%s", err, out.String())
	}
	path := filepath.Join(vault, "10-sessions", fakeSessionUUID+".md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["session_id"] != fakeSessionUUID {
		t.Errorf("session_id = %v", a.FrontMatter["session_id"])
	}
	if a.FrontMatter["source"] != "claude-code" {
		t.Errorf("source = %v", a.FrontMatter["source"])
	}
	if err := schema.Validate("session", a.FrontMatter); err != nil {
		t.Errorf("schema: %v", err)
	}
}

func TestCreateSession_StampsActiveThreadFromFlag(t *testing.T) {
	vault := setupVault(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "session", "--session-id", fakeSessionUUID, "--active-thread", "research-ducklake"})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "10-sessions", fakeSessionUUID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	related, _ := a.FrontMatter["related"].([]any)
	if len(related) != 1 || related[0] != "[[thread.research-ducklake]]" {
		t.Errorf("related = %v", related)
	}
}

func TestCreateSession_IdempotentOnReRun(t *testing.T) {
	vault := setupVault(t)

	first := newRootCmd()
	first.SetArgs([]string{"create", "session", "--session-id", fakeSessionUUID,
		"--started-at", "2026-05-06T12:00:00Z"})
	first.SetOut(&bytes.Buffer{})
	if err := first.Execute(); err != nil {
		t.Fatalf("first: %v", err)
	}
	path := filepath.Join(vault, "10-sessions", fakeSessionUUID+".md")
	c1, _ := os.ReadFile(path)

	second := newRootCmd()
	second.SetArgs([]string{"create", "session", "--session-id", fakeSessionUUID,
		"--started-at", "2026-05-06T12:00:00Z"})
	second.SetOut(&bytes.Buffer{})
	if err := second.Execute(); err != nil {
		t.Fatalf("second: %v", err)
	}
	c2, _ := os.ReadFile(path)
	if string(c1) != string(c2) {
		t.Errorf("file rewritten on idempotent re-run")
	}
}

func TestCreateSession_DriftErrorsWithoutUpdate(t *testing.T) {
	setupVault(t)

	first := newRootCmd()
	first.SetArgs([]string{"create", "session", "--session-id", fakeSessionUUID,
		"--started-at", "2026-05-06T12:00:00Z"})
	first.SetOut(&bytes.Buffer{})
	if err := first.Execute(); err != nil {
		t.Fatalf("first: %v", err)
	}

	second := newRootCmd()
	second.SetArgs([]string{"create", "session", "--session-id", fakeSessionUUID,
		"--started-at", "2026-05-06T13:00:00Z"})
	second.SetErr(&bytes.Buffer{})
	err := second.Execute()
	if err == nil {
		t.Fatal("expected drift error")
	}
	if !strings.Contains(err.Error(), "--update") {
		t.Errorf("error should suggest --update: %q", err.Error())
	}
}

func TestCreateSession_UpdateRewritesOnDrift(t *testing.T) {
	vault := setupVault(t)

	first := newRootCmd()
	first.SetArgs([]string{"create", "session", "--session-id", fakeSessionUUID,
		"--started-at", "2026-05-06T12:00:00Z"})
	first.SetOut(&bytes.Buffer{})
	if err := first.Execute(); err != nil {
		t.Fatalf("first: %v", err)
	}

	upd := newRootCmd()
	upd.SetArgs([]string{"create", "session", "--session-id", fakeSessionUUID,
		"--started-at", "2026-05-06T13:00:00Z", "--update"})
	upd.SetOut(&bytes.Buffer{})
	if err := upd.Execute(); err != nil {
		t.Fatalf("update: %v", err)
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "10-sessions", fakeSessionUUID+".md"))
	if a.FrontMatter["started_at"] != "2026-05-06T13:00:00Z" {
		t.Errorf("started_at not updated: %v", a.FrontMatter["started_at"])
	}
}

func TestCreateSession_RejectsUnknownSource(t *testing.T) {
	setupVault(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "session", "--session-id", fakeSessionUUID, "--source", "vscode"})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateSession_JSON(t *testing.T) {
	vault := setupVault(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "session", "--session-id", fakeSessionUUID,
		"--active-thread", "research-ducklake", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("emit: %v", err)
	}
	var got struct {
		ID, Path string
		Related  []string
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if got.ID != fakeSessionUUID {
		t.Errorf("id = %q", got.ID)
	}
	if got.Path != filepath.Join(vault, "10-sessions", fakeSessionUUID+".md") {
		t.Errorf("path = %q", got.Path)
	}
	if len(got.Related) != 1 || got.Related[0] != "[[thread.research-ducklake]]" {
		t.Errorf("related = %v", got.Related)
	}
}

func TestInstallFireSessionStart_WritesSession(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("ANVIL_STATE_DIR", t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"install", "fire-session-start"})
	cmd.SetIn(strings.NewReader(`{"session_id":"` + fakeSessionUUID + `","source":"startup","cwd":"/tmp","hook_event_name":"SessionStart"}`))
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("hook: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "10-sessions", fakeSessionUUID+".md"))
	if err != nil {
		t.Fatalf("missing session: %v", err)
	}
	if a.FrontMatter["source"] != "claude-code" {
		t.Errorf("source = %v, want claude-code", a.FrontMatter["source"])
	}
}

func TestCreateInbox_WritesFile(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "inbox", "--title", "streaming feels laggy"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if len(entries) != 1 {
		t.Errorf("expected 1 inbox file, got %d", len(entries))
	}
}

func TestCreateInbox_WithBody(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "inbox", "--title", "x", "--body", "stub body"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 inbox file")
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "00-inbox", entries[0].Name()))
	if !strings.Contains(a.Body, "stub body") {
		t.Errorf("body = %q", a.Body)
	}
}

func TestCreate_Issue_RejectsUnknownDomain(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue",
		"--title", "Fix X",
		"--description", "y",
		"--tags", "domain/quantum-physics"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected failure for unknown domain")
	}
	if !strings.Contains(errOut.String(), "unknown_facet_value") {
		t.Errorf("missing code in stderr: %q", errOut.String())
	}
	if !strings.Contains(errOut.String(), "--allow-new-facet=domain") {
		t.Errorf("missing fix line: %q", errOut.String())
	}
}

func TestCreate_Issue_AllowNewFacetSucceeds(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue",
		"--title", "Fix X",
		"--description", "y",
		"--tags", "domain/quantum-physics",
		"--allow-new-facet=domain"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected success: %v", err)
	}
}

func TestCreate_Issue_SuggestsContainmentMatch(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue", "--title", "seed",
		"--description", "y", "--tags", "domain/dbt", "--allow-new-facet=domain"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"create", "issue", "--title", "Other",
		"--description", "y", "--tags", "domain/dbt-testing"})
	var errOut bytes.Buffer
	cmd2.SetErr(&errOut)
	if err := cmd2.Execute(); err == nil {
		t.Fatal("expected rejection")
	}
	if !strings.Contains(errOut.String(), "suggest: domain/dbt") {
		t.Errorf("missing suggest line: %q", errOut.String())
	}
}

func TestCreate_Issue_Idempotent_ReturnsAlreadyExists(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	args := []string{"create", "issue",
		"--title", "Fix login bug",
		"--description", "test description",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
		"--json",
	}

	cmd1 := newRootCmd()
	cmd1.SetArgs(args)
	var out1 bytes.Buffer
	cmd1.SetOut(&out1)
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("first create: %v", err)
	}
	var first map[string]string
	if err := json.Unmarshal(out1.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first["status"] != "created" {
		t.Errorf("first status = %q want created", first["status"])
	}

	path := filepath.Join(vault, "70-issues", "foo.fix-login-bug.md")
	statBefore, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	cmd2 := newRootCmd()
	cmd2.SetArgs(args)
	var out2 bytes.Buffer
	cmd2.SetOut(&out2)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second create: %v", err)
	}
	var second map[string]string
	if err := json.Unmarshal(out2.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if second["status"] != "already_exists" {
		t.Errorf("second status = %q want already_exists", second["status"])
	}
	if second["path"] != first["path"] {
		t.Errorf("path drifted: %q vs %q", second["path"], first["path"])
	}

	// no -2 sibling
	if _, err := os.Stat(filepath.Join(vault, "70-issues", "foo.fix-login-bug-2.md")); !os.IsNotExist(err) {
		t.Errorf("unexpected -2 sibling: err=%v", err)
	}

	statAfter, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !statBefore.ModTime().Equal(statAfter.ModTime()) {
		t.Errorf("mtime changed on idempotent re-run")
	}
}

func TestCreate_Issue_DriftRefusedWithoutUpdate(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	mk := func(desc string) *cobra.Command {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"create", "issue",
			"--title", "Fix login bug",
			"--description", desc,
			"--tags", "domain/dev-tools",
			"--allow-new-facet=domain",
		})
		return cmd
	}
	cmd1 := mk("first description")
	cmd1.SetOut(new(bytes.Buffer))
	if err := cmd1.Execute(); err != nil {
		t.Fatal(err)
	}
	cmd2 := mk("second description")
	var out bytes.Buffer
	cmd2.SetOut(&out)
	cmd2.SetErr(&out)
	err := cmd2.Execute()
	if err == nil {
		t.Fatal("expected drift error")
	}
	if !errors.Is(err, ErrCreateDrift) {
		t.Errorf("err = %v, want ErrCreateDrift", err)
	}
	if !strings.Contains(err.Error(), "description") {
		t.Errorf("err should name 'description': %v", err)
	}
}

func TestCreate_Issue_TagReorder_NoDrift(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	mk := func(tags string) *cobra.Command {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"create", "issue",
			"--title", "Fix login bug",
			"--description", "d",
			"--tags", tags,
			"--allow-new-facet=domain",
		})
		return cmd
	}
	if err := mk("domain/dev-tools,domain/cli").Execute(); err != nil {
		t.Fatal(err)
	}
	if err := mk("domain/cli,domain/dev-tools").Execute(); err != nil {
		t.Errorf("expected no drift on tag reorder, got: %v", err)
	}
}

func TestCreate_Issue_BodyDrift_RefusedWithoutUpdate(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	mk := func(body string) *cobra.Command {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"create", "issue",
			"--title", "Fix login bug",
			"--description", "d",
			"--body", body,
			"--tags", "domain/dev-tools",
			"--allow-new-facet=domain",
		})
		return cmd
	}
	if err := mk("original body").Execute(); err != nil {
		t.Fatal(err)
	}
	err := mk("different body").Execute()
	if !errors.Is(err, ErrCreateDrift) {
		t.Errorf("err = %v, want ErrCreateDrift", err)
	}
}

func TestCreate_Issue_UpdateRewritesOnDrift(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd1 := newRootCmd()
	cmd1.SetArgs([]string{"create", "issue",
		"--title", "Fix login bug",
		"--description", "first",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	})
	if err := cmd1.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "70-issues", "foo.fix-login-bug.md")
	first, _ := core.LoadArtifact(path)
	originalCreated := first.FrontMatter["created"]

	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"create", "issue",
		"--title", "Fix login bug",
		"--description", "second",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
		"--update",
		"--json",
	})
	var out bytes.Buffer
	cmd2.SetOut(&out)
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("update: %v", err)
	}
	var resp map[string]string
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "updated" {
		t.Errorf("status = %q, want updated", resp["status"])
	}

	after, _ := core.LoadArtifact(path)
	if after.FrontMatter["description"] != "second" {
		t.Errorf("description not rewritten: %v", after.FrontMatter["description"])
	}
	if after.FrontMatter["created"] != originalCreated {
		t.Errorf("created changed: %v -> %v", originalCreated, after.FrontMatter["created"])
	}
}

func TestCreate_Issue_UpdateWithoutDrift_NoRewrite(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	args := []string{"create", "issue",
		"--title", "Fix login bug",
		"--description", "same",
		"--tags", "domain/dev-tools",
		"--allow-new-facet=domain",
	}
	cmd1 := newRootCmd()
	cmd1.SetArgs(args)
	if err := cmd1.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(vault, "70-issues", "foo.fix-login-bug.md")
	statBefore, _ := os.Stat(path)

	cmd2 := newRootCmd()
	cmd2.SetArgs(append(args, "--update", "--json"))
	var out bytes.Buffer
	cmd2.SetOut(&out)
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}
	var resp map[string]string
	_ = json.Unmarshal(out.Bytes(), &resp)
	if resp["status"] != "already_exists" {
		t.Errorf("status = %q, want already_exists (no drift, no rewrite)", resp["status"])
	}
	statAfter, _ := os.Stat(path)
	if !statBefore.ModTime().Equal(statAfter.ModTime()) {
		t.Errorf("mtime changed despite no drift")
	}
}

func TestCreate_DriftError_FormatsScalar(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	mk := func(desc string) *cobra.Command {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"create", "issue",
			"--title", "Fix login bug",
			"--description", desc,
			"--tags", "domain/dev-tools",
			"--allow-new-facet=domain",
		})
		return cmd
	}
	_ = mk("first description").Execute()

	cmd := mk("second description")
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected drift error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "already exists with different description") {
		t.Errorf("missing field name in: %s", msg)
	}
	if !strings.Contains(msg, `existing: "first description"`) {
		t.Errorf("missing existing scalar in: %s", msg)
	}
	if !strings.Contains(msg, `new:      "second description"`) {
		t.Errorf("missing new scalar in: %s", msg)
	}
	if !strings.Contains(msg, "retry with --update") {
		t.Errorf("missing remediation hint in: %s", msg)
	}
}

func TestCreate_DriftError_FormatsTagsArray(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	mk := func(tags string) *cobra.Command {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"create", "issue",
			"--title", "Fix login bug",
			"--description", "d",
			"--tags", tags,
			"--allow-new-facet=domain",
		})
		return cmd
	}
	_ = mk("domain/dev-tools").Execute()
	err := mk("domain/dev-tools,domain/cli").Execute()
	if err == nil || !strings.Contains(err.Error(), "[domain/dev-tools, domain/cli]") {
		t.Errorf("array drift not rendered as [a, b]: %v", err)
	}
}

func TestCreate_DriftError_FormatsBodyTruncated(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	long := strings.Repeat("a", 200)
	mk := func(body string) *cobra.Command {
		cmd := newRootCmd()
		cmd.SetArgs([]string{"create", "issue",
			"--title", "Fix login bug",
			"--description", "d",
			"--body", body,
			"--tags", "domain/dev-tools",
			"--allow-new-facet=domain",
		})
		return cmd
	}
	_ = mk(long).Execute()
	err := mk(strings.Repeat("b", 200)).Execute()
	if err == nil {
		t.Fatal("expected drift error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "different body") {
		t.Errorf("missing 'body' field name: %s", msg)
	}
	if !strings.Contains(msg, "…") {
		t.Errorf("expected truncation ellipsis: %s", msg)
	}
}

func TestCreate_Decision_StaysAppendOnly(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	args := []string{"create", "decision",
		"--topic", "db",
		"--title", "Pick Postgres",
		"--description", "d",
		"--tags", "domain/infra,activity/research",
		"--allow-new-facet=domain", "--allow-new-facet=activity",
	}
	for i := 1; i <= 2; i++ {
		c := newRootCmd()
		c.SetArgs(args)
		if err := c.Execute(); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	dir := filepath.Join(vault, "30-decisions")
	entries, _ := os.ReadDir(dir)
	count := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "db.") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("decision should be append-only: have %d files, want 2", count)
	}
}

func TestCreate_AllSlugTypes_Idempotent(t *testing.T) {
	type tc struct {
		name string
		args []string
	}
	cases := []tc{
		{"thread", []string{"create", "thread", "--title", "auth retries", "--description", "d",
			"--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"}},
		{"learning", []string{"create", "learning", "--title", "slogger gotcha", "--description", "d",
			"--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"}},
		{"sweep", []string{"create", "sweep", "--title", "drop python2", "--description", "d",
			"--scope", "py", "--breaking=false",
			"--tags", "domain/dev-tools", "--allow-new-facet=domain"}},
		{"inbox", []string{"create", "inbox", "--title", "random thought", "--description", "d",
			"--tags", "domain/dev-tools", "--allow-new-facet=domain"}},
	}
	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			setupVault(t)
			repo := setupGitRepo(t, "git@github.com:acme/foo.git")
			t.Setenv("HOME", t.TempDir())
			t.Chdir(repo)

			args := append([]string{}, tcase.args...)
			args = append(args, "--json")
			c1 := newRootCmd()
			c1.SetArgs(args)
			var b1 bytes.Buffer
			c1.SetOut(&b1)
			if err := c1.Execute(); err != nil {
				t.Fatal(err)
			}
			c2 := newRootCmd()
			c2.SetArgs(args)
			var b2 bytes.Buffer
			c2.SetOut(&b2)
			if err := c2.Execute(); err != nil {
				t.Fatal(err)
			}
			var resp map[string]string
			if err := json.Unmarshal(b2.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			if resp["status"] != "already_exists" {
				t.Errorf("%s: status = %q, want already_exists", tcase.name, resp["status"])
			}
		})
	}
}

// TestCreate_DescriptionPreflight_RejectsOversize asserts the CLI rejects an
// over-cap --description before any artifact is written. Exercises the AC3
// pre-flight path: error names the limit + actual length, no stub left behind.
func TestCreate_DescriptionPreflight_RejectsOversize(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	long := strings.Repeat("a", 121)
	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "issue", "--title", "oversize", "--description", long, "--tags", "domain/dev-tools", "--allow-new-facet=domain"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected pre-flight error for over-cap description")
	}
	msg := err.Error()
	if !strings.Contains(msg, "121") || !strings.Contains(msg, "120") {
		t.Errorf("error must name actual length and cap; got: %s", msg)
	}
	// No file should be written under the issues dir.
	entries, readErr := os.ReadDir(filepath.Join(vault, "70-issues"))
	if readErr != nil {
		t.Fatalf("reading issues dir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files written on pre-flight failure, got %d", len(entries))
	}
}
