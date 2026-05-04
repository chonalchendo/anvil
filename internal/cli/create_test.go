package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	cmd.SetArgs([]string{"create", "issue", "--title", "Fix login bug"})
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
	cmd.SetArgs([]string{"create", "milestone", "--title", "CLI substrate"})
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
	cmd.SetArgs([]string{"create", "issue", "--title", "x", "--json"})
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
	cmd.SetArgs([]string{"create", "decision", "--title", "use jwt", "--topic", "auth"})
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
		"--issue", "[[issue.foo.streaming]]",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create: %v\n%s", err, out.String())
	}
	// Confirm the file exists and validates end-to-end.
	path := filepath.Join(vault, "80-plans", "foo.streaming-token-counter.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}
	p, err := core.LoadPlan(path)
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}
	if err := core.ValidatePlan(p); err != nil {
		t.Errorf("freshly-created plan should validate, got: %v", err)
	}
}

func TestCreate_Thread_WritesValidFile(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir()) // not a git repo — thread needs no project

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "thread", "--title", "Research ducklake"})
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
	cmd.SetArgs([]string{"create", "learning", "--title", "Postgres FK locks block writes"})
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
	cmd.SetArgs([]string{"create", "issue", "--title", "x", "--body", "## Context\n\nFrom flag."})
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
	cmd.SetArgs([]string{"create", "issue", "--title", "x"})
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
		"--issue", "[[issue.foo.streaming]]",
		"--body", body,
	})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create plan: %v\n%s", err, stderr.String())
	}
	a, _ := core.LoadArtifact(filepath.Join(vault, "80-plans", "foo.author-body.md"))
	if !strings.Contains(a.Body, "Author-supplied T1") {
		t.Errorf("body did not replace T1 seed: %q", a.Body)
	}
}

func TestCreatePlan_BodyTooShort_FailsValidation(t *testing.T) {
	setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{
		"create", "plan",
		"--title", "Short",
		"--issue", "[[issue.foo.x]]",
		"--body", "## Task: T1\n\ntoo short.\n",
	})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(&stderr)
	if err := cmd.Execute(); err == nil {
		t.Error("expected ValidatePlan to reject truncated body")
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
	cmd.SetArgs([]string{"create", "milestone", "--title", "CLI substrate"})
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
	cmd.SetArgs([]string{"create", "product-design", "--title", "Anvil product design"})
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

func TestCreate_ProductDesign_RefusesOverwrite(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "product-design", "--title", "First"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first create: %v", err)
	}
	path := filepath.Join(vault, "05-projects", "foo", "product-design.md")
	first, _ := os.ReadFile(path)

	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"create", "product-design", "--title", "Second"})
	var stderr bytes.Buffer
	cmd2.SetErr(&stderr)
	cmd2.SetOut(&stderr)
	if err := cmd2.Execute(); err == nil {
		t.Error("expected error on duplicate product-design")
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %v, want mention of existing", err)
	}
	after, _ := os.ReadFile(path)
	if string(first) != string(after) {
		t.Error("first file mutated after second create attempt")
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
	cmd.SetArgs([]string{"create", "sweep", "--title", "CLI rename", "--scope", "cli", "--breaking"})
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
	cmd.SetArgs([]string{"create", "sweep", "--title", "Docs polish", "--scope", "docs", "--breaking=false"})
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
	cmd.SetArgs([]string{"create", "system-design", "--title", "Anvil system design"})
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
