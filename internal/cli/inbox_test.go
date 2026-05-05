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

func TestInbox_Add(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "add", "--title", "streaming feels laggy"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if len(entries) != 1 {
		t.Errorf("expected 1 inbox file, got %d", len(entries))
	}
}

func TestInbox_List(t *testing.T) {
	setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "list"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestInbox_Promote_Issue(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "broken thing", "--suggested-project", "foo"})
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if len(entries) != 1 {
		t.Fatal("expected 1 inbox file")
	}
	inboxID := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", inboxID, "--as", "issue"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote: %v", err)
	}

	inboxPath := filepath.Join(vault, "00-inbox", inboxID+".md")
	a, err := core.LoadArtifact(inboxPath)
	if err != nil {
		t.Fatalf("inbox file should persist: %v", err)
	}
	if got := a.FrontMatter["status"]; got != "promoted" {
		t.Errorf("status = %v, want promoted", got)
	}
	if got := a.FrontMatter["promoted_type"]; got != "issue" {
		t.Errorf("promoted_type = %v, want issue", got)
	}
	if got, _ := a.FrontMatter["promoted_to"].(string); got == "" {
		t.Error("promoted_to should be set")
	}
	if _, ok := a.FrontMatter["updated"]; !ok {
		t.Error("updated should be set")
	}

	issuePath := filepath.Join(vault, "70-issues", "foo.broken-thing.md")
	if _, err := core.LoadArtifact(issuePath); err != nil {
		t.Fatalf("expected issue at %s: %v", issuePath, err)
	}
}

func TestInbox_Promote_Discard(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", id, "--as", "discard"})
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

func TestInboxPromote_AsThread(t *testing.T) {
	vault := setupVault(t)
	repo := setupGitRepo(t, "git@github.com:acme/foo.git")
	t.Setenv("HOME", t.TempDir())
	t.Chdir(repo)

	var buf bytes.Buffer
	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "Ducklake?", "--json"})
	add.SetOut(&buf)
	if err := add.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct{ ID, Path string }
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	promote := newRootCmd()
	promote.SetArgs([]string{"inbox", "promote", result.ID, "--as", "thread"})
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

func TestInboxPromote_ToLearning(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "add", "--title", "FK locks block writes", "--json"})
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
	cmd.SetArgs([]string{"inbox", "promote", added.ID, "--as", "learning"})
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

func TestInboxPromote_RequiresAsFlag(t *testing.T) {
	vault := setupVault(t)
	_ = vault
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x", "--suggested-type", "issue"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", id})
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

func TestInboxPromote_InvalidAsValue(t *testing.T) {
	vault := setupVault(t)
	_ = vault
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", id, "--as", "isue"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		`invalid value "isue" for --as`,
		"valid values: issue, thread, design, learning, discard",
		"corrected:    anvil inbox promote " + id + " --as issue",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q\nfull error:\n%s", want, msg)
		}
	}
}

func TestInboxPromote_Idempotent(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"inbox", "promote", id, "--as", "thread"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first promote: %v", err)
	}

	second := newRootCmd()
	second.SetArgs([]string{"inbox", "promote", id, "--as", "thread"})
	var out bytes.Buffer
	second.SetOut(&out)
	if err := second.Execute(); err != nil {
		t.Fatalf("second promote: %v", err)
	}
	if !strings.HasPrefix(out.String(), "already promoted ") {
		t.Errorf("output = %q, want 'already promoted ...'", out.String())
	}

	threads, _ := os.ReadDir(filepath.Join(vault, "60-threads"))
	if len(threads) != 1 {
		t.Errorf("expected exactly 1 thread file, got %d", len(threads))
	}
}

func TestInboxDiscard_Idempotent(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"inbox", "promote", id, "--as", "discard"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first discard: %v", err)
	}

	second := newRootCmd()
	second.SetArgs([]string{"inbox", "promote", id, "--as", "discard"})
	var out bytes.Buffer
	second.SetOut(&out)
	if err := second.Execute(); err != nil {
		t.Fatalf("second discard: %v", err)
	}
	if !strings.HasPrefix(out.String(), "already discarded ") {
		t.Errorf("output = %q, want 'already discarded ...'", out.String())
	}
}

func TestInboxPromote_MismatchedAs(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"inbox", "promote", id, "--as", "thread"})
	if err := first.Execute(); err != nil {
		t.Fatalf("first: %v", err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", id, "--as", "learning"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	msg := err.Error()
	for _, want := range []string{
		`invalid value "learning" for --as`,
		"valid values: thread",
		"corrected:    anvil inbox promote " + id + " --as thread",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q\nfull:\n%s", want, msg)
		}
	}
}

func TestInboxPromote_OnDropped(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"inbox", "promote", id, "--as", "discard"})
	first.Execute()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", id, "--as", "issue"})
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

func TestInboxDiscard_OnPromoted(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	first := newRootCmd()
	first.SetArgs([]string{"inbox", "promote", id, "--as", "thread"})
	first.Execute()

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "promote", id, "--as", "discard"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "already promoted to thread") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestInboxAdd_WithBody(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	cmd := newRootCmd()
	cmd.SetArgs([]string{"inbox", "add", "--title", "x", "--body", "stub body"})
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
