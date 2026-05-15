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

func TestPromote_TopLevel_Issue(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "broken thing", "--suggested-project", "foo"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if len(entries) != 1 {
		t.Fatalf("expected 1 inbox file, got %d", len(entries))
	}
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "issue", "--tags", "domain/dev-tools", "--allow-new-facet=domain"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote: %v", err)
	}
	a, err := core.LoadArtifact(filepath.Join(vault, "00-inbox", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if a.FrontMatter["status"] != "promoted" {
		t.Errorf("status = %v", a.FrontMatter["status"])
	}
}

func TestPromote_TopLevel_Idempotent(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"promote", id, "--as", "thread", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first: %v", err)
	}
	second := newRootCmd()
	second.SetArgs([]string{"promote", id, "--as", "thread"})
	var out bytes.Buffer
	second.SetOut(&out)
	if err := second.Execute(); err != nil {
		t.Fatalf("second: %v", err)
	}
	if !strings.HasPrefix(out.String(), "already promoted ") {
		t.Errorf("output = %q", out.String())
	}
}

func TestPromote_TopLevel_InvalidAsSuggestsTopLevelCommand(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "isue"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "anvil promote ") {
		t.Errorf("corrected line should reference top-level promote: %q", err.Error())
	}
}

func TestPromote_TopLevel_JSON(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "thread", "--json", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote: %v", err)
	}
	var r struct {
		ID, Status string
		TargetID   *string `json:"target_id"`
	}
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if r.Status != "promoted" || r.TargetID == nil || *r.TargetID == "" {
		t.Errorf("unexpected: %+v", r)
	}
}

func TestPromote_AsThread(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	var buf bytes.Buffer
	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "Ducklake?", "--json"})
	add.SetOut(&buf)
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct{ ID, Path string }
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	promote := newRootCmd()
	promote.SetArgs([]string{"promote", result.ID, "--as", "thread", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	if err := promote.Execute(); err != nil {
		t.Fatalf("promote: %v", err)
	}

	a, err := core.LoadArtifact(result.Path)
	if err != nil {
		t.Fatalf("inbox file should persist: %v", err)
	}
	if got := a.FrontMatter["status"]; got != "promoted" {
		t.Errorf("status = %v, want promoted", got)
	}
	if got := a.FrontMatter["promoted_type"]; got != "thread" {
		t.Errorf("promoted_type = %v, want thread", got)
	}
	threadPath := filepath.Join(vault, "60-threads", "ducklake.md")
	if _, err := core.LoadArtifact(threadPath); err != nil {
		t.Fatalf("expected thread at %s: %v", threadPath, err)
	}
}

func TestPromote_ToLearning(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "inbox", "--title", "FK locks block writes", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inbox add: %v", err)
	}
	var added struct{ ID, Path string }
	if err := json.Unmarshal(out.Bytes(), &added); err != nil {
		t.Fatal(err)
	}

	cmd = newRootCmd()
	cmd.SetArgs([]string{"promote", added.ID, "--as", "learning", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote: %v", err)
	}

	a, err := core.LoadArtifact(added.Path)
	if err != nil {
		t.Fatalf("inbox file should persist: %v", err)
	}
	if got := a.FrontMatter["status"]; got != "promoted" {
		t.Errorf("status = %v, want promoted", got)
	}
	learningPath := filepath.Join(vault, "20-learnings", "fk-locks-block-writes.md")
	la, err := core.LoadArtifact(learningPath)
	if err != nil {
		t.Fatalf("expected learning at %s: %v", learningPath, err)
	}
	if la.FrontMatter["status"] != "draft" {
		t.Errorf("learning status = %v, want draft", la.FrontMatter["status"])
	}
}

func TestPromote_Discard(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "discard"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("discard: %v", err)
	}

	a, err := core.LoadArtifact(filepath.Join(vault, "00-inbox", id+".md"))
	if err != nil {
		t.Fatalf("inbox file should persist: %v", err)
	}
	if got := a.FrontMatter["status"]; got != "dropped" {
		t.Errorf("status = %v, want dropped", got)
	}
	if _, ok := a.FrontMatter["promoted_to"]; ok {
		t.Error("promoted_to should be absent on discard")
	}
	if _, ok := a.FrontMatter["promoted_type"]; ok {
		t.Error("promoted_type should be absent on discard")
	}
	issues, _ := os.ReadDir(filepath.Join(vault, "70-issues"))
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %d", len(issues))
	}
}

func TestPromote_RequiresAsFlag(t *testing.T) {
	vault := setupVault(t)
	_ = vault
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x", "--suggested-type", "issue"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id})
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SilenceUsage = true
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error: --as is required")
	}
	if !strings.Contains(err.Error(), `required flag(s) "as" not set`) {
		t.Errorf("error = %q, want cobra required-flag message", err.Error())
	}
}

func TestPromote_InvalidAsValue(t *testing.T) {
	vault := setupVault(t)
	_ = vault
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "isue"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		`invalid value "isue" for --as`,
		"valid values: issue, thread, learning, discard",
		"corrected:    anvil promote " + id + " --as issue",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q\nfull error:\n%s", want, msg)
		}
	}
}

func TestPromote_DiscardIdempotent(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"promote", id, "--as", "discard"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first discard: %v", err)
	}

	second := newRootCmd()
	second.SetArgs([]string{"promote", id, "--as", "discard"})
	var out bytes.Buffer
	second.SetOut(&out)
	if err := second.Execute(); err != nil {
		t.Fatalf("second discard: %v", err)
	}
	if !strings.HasPrefix(out.String(), "already discarded ") {
		t.Errorf("output = %q, want 'already discarded ...'", out.String())
	}
}

func TestPromote_MismatchedAs(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"promote", id, "--as", "thread", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first: %v", err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "learning"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	msg := err.Error()
	for _, want := range []string{
		`invalid value "learning" for --as`,
		"valid values: thread",
		"corrected:    anvil promote " + id + " --as thread",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q\nfull:\n%s", want, msg)
		}
	}
}

func TestPromote_OnDropped(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"promote", id, "--as", "discard"})
	first.Execute()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "issue"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cannot promote a dropped entry") {
		t.Errorf("error = %q", err.Error())
	}
	if strings.Contains(err.Error(), "corrected:") {
		t.Errorf("dropped→promote error must not include 'corrected:' line:\n%s", err.Error())
	}
}

func TestPromote_DiscardOnPromoted(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"promote", id, "--as", "thread", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	first.Execute()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "discard"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "already promoted to thread") {
		t.Errorf("error = %q", err.Error())
	}
}

type promoteJSONResult struct {
	ID         string  `json:"id"`
	TargetID   *string `json:"target_id"`
	TargetType *string `json:"target_type"`
	Status     string  `json:"status"`
	Path       *string `json:"path"`
}

func runPromoteJSON(t *testing.T, args ...string) promoteJSONResult {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs(append([]string{"promote"}, args...))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote %v: %v", args, err)
	}
	var r promoteJSONResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &r); err != nil {
		t.Fatalf("unmarshal %q: %v", out.String(), err)
	}
	return r
}

func TestPromote_JSON_AlreadyPromoted(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	runPromoteJSON(t, id, "--as", "thread", "--json", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity")
	r := runPromoteJSON(t, id, "--as", "thread", "--json")
	if r.Status != "already_promoted" {
		t.Errorf("status = %q, want already_promoted", r.Status)
	}
	if r.TargetType == nil || *r.TargetType != "thread" {
		t.Errorf("target_type = %v, want thread", r.TargetType)
	}
}

func TestPromote_JSON_Discarded(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	r := runPromoteJSON(t, id, "--as", "discard", "--json")
	if r.Status != "discarded" {
		t.Errorf("status = %q, want discarded", r.Status)
	}
	if r.TargetID != nil || r.TargetType != nil || r.Path != nil {
		t.Errorf("discard result must have null target_id/target_type/path: %+v", r)
	}
}

func TestPromote_JSON_AlreadyDiscarded(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	runPromoteJSON(t, id, "--as", "discard", "--json")
	r := runPromoteJSON(t, id, "--as", "discard", "--json")
	if r.Status != "already_discarded" {
		t.Errorf("status = %q, want already_discarded", r.Status)
	}
	if r.TargetID != nil || r.TargetType != nil || r.Path != nil {
		t.Errorf("already-discarded result must have null target fields: %+v", r)
	}
}

func TestPromote_CopiesInboxBody(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	const body = "this body must travel\n"
	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "carries body", "--suggested-project", "foo", "--body", body})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "issue", "--tags", "domain/dev-tools", "--allow-new-facet=domain", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote: %v", err)
	}
	var r promoteJSONResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &r); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if r.Path == nil {
		t.Fatal("promote did not return target path")
	}
	a, err := core.LoadArtifact(*r.Path)
	if err != nil {
		t.Fatalf("load promoted: %v", err)
	}
	if strings.TrimSpace(a.Body) != strings.TrimSpace(body) {
		t.Errorf("promoted body = %q, want %q", a.Body, body)
	}
}

// TestPromote_PreExistingTargetGetsSuffix documents the NextID uniqueness
// contract: when the deterministic target path is occupied, NextID hands out a
// suffixed ID (e.g. `-2`), so promote can never overwrite a pre-existing file.
// This is why promote.go does not need a defensive os.Stat guard before Save.
func TestPromote_PreExistingTargetGetsSuffix(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	threadsDir := filepath.Join(vault, "60-threads")
	if err := os.MkdirAll(threadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	preExistingPath := filepath.Join(threadsDir, "collide.md")
	preExistingBody := "do not touch me\n"
	preExisting := &core.Artifact{
		Path: preExistingPath,
		FrontMatter: map[string]any{
			"id":      "collide",
			"type":    "thread",
			"title":   "pre-existing",
			"status":  "active",
			"created": "2026-01-01",
			"updated": "2026-01-01",
			"tags":    []any{"domain/dev-tools", "activity/research"},
		},
		Body: preExistingBody,
	}
	if err := preExisting.Save(); err != nil {
		t.Fatalf("pre-create: %v", err)
	}

	add := newRootCmd()
	add.SetArgs([]string{"create", "inbox", "--title", "collide"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"promote", id, "--as", "thread", "--json", "--tags", "domain/dev-tools,activity/research", "--allow-new-facet=domain", "--allow-new-facet=activity"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote: %v", err)
	}
	var r promoteJSONResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &r); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if r.TargetID == nil || *r.TargetID != "collide-2" {
		t.Errorf("target_id = %v, want collide-2", r.TargetID)
	}

	preAfter, err := core.LoadArtifact(preExistingPath)
	if err != nil {
		t.Fatalf("reload pre-existing: %v", err)
	}
	if strings.TrimSpace(preAfter.Body) != strings.TrimSpace(preExistingBody) {
		t.Errorf("pre-existing body mutated: got %q, want %q", preAfter.Body, preExistingBody)
	}
}

func TestPromote_Issue_RequiresTags(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"create", "inbox", "--title", "thought", "--suggested-project", "anvil"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"promote", id, "--as", "issue"})
	var errOut bytes.Buffer
	cmd2.SetErr(&errOut)
	cmd2.SetOut(&errOut)
	if err := cmd2.Execute(); err == nil {
		t.Fatal("expected schema rejection for promote without --tags")
	}

	cmd3 := newRootCmd()
	cmd3.SetArgs([]string{
		"promote", id, "--as", "issue",
		"--tags", "domain/dev-tools", "--allow-new-facet=domain",
	})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("expected success: %v", err)
	}
}
