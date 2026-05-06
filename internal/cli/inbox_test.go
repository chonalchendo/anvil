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
		"corrected:    anvil promote " + id + " --as issue",
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
		"corrected:    anvil promote " + id + " --as thread",
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

type promoteResult struct {
	ID         string  `json:"id"`
	TargetID   *string `json:"target_id"`
	TargetType *string `json:"target_type"`
	Status     string  `json:"status"`
	Path       *string `json:"path"`
}

func runPromoteJSON(t *testing.T, args ...string) promoteResult {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs(append([]string{"inbox", "promote"}, args...))
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("promote %v: %v", args, err)
	}
	var r promoteResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &r); err != nil {
		t.Fatalf("unmarshal %q: %v", out.String(), err)
	}
	return r
}

func TestInboxPromote_JSON_Promoted(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	r := runPromoteJSON(t, id, "--as", "thread", "--json")
	if r.Status != "promoted" {
		t.Errorf("status = %q, want promoted", r.Status)
	}
	if r.TargetType == nil || *r.TargetType != "thread" {
		t.Errorf("target_type = %v, want thread", r.TargetType)
	}
	if r.TargetID == nil || *r.TargetID == "" {
		t.Error("target_id should be non-empty")
	}
	if r.Path == nil || !filepath.IsAbs(*r.Path) {
		t.Errorf("path = %v, want absolute", r.Path)
	}
}

func TestInboxPromote_JSON_AlreadyPromoted(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
	add.Execute()
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	id := strings.TrimSuffix(entries[0].Name(), ".md")

	runPromoteJSON(t, id, "--as", "thread", "--json")
	r := runPromoteJSON(t, id, "--as", "thread", "--json")
	if r.Status != "already_promoted" {
		t.Errorf("status = %q, want already_promoted", r.Status)
	}
	if r.TargetType == nil || *r.TargetType != "thread" {
		t.Errorf("target_type = %v, want thread", r.TargetType)
	}
}

func TestInboxPromote_JSON_Discarded(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
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

func TestInboxPromote_JSON_AlreadyDiscarded(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	add := newRootCmd()
	add.SetArgs([]string{"inbox", "add", "--title", "x"})
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

func TestInboxList_DefaultsToRaw(t *testing.T) {
	vault := setupVault(t)
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	// Seed: one raw, one promoted, one dropped.
	for _, title := range []string{"raw-one", "to-promote", "to-drop"} {
		add := newRootCmd()
		add.SetArgs([]string{"inbox", "add", "--title", title})
		if err := add.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if len(entries) != 3 {
		t.Fatalf("expected 3 inbox files, got %d", len(entries))
	}
	ids := make([]string, 0, 3)
	for _, e := range entries {
		ids = append(ids, strings.TrimSuffix(e.Name(), ".md"))
	}
	// ids are sorted by filename; titles encode position.
	promoteCmd := newRootCmd()
	promoteCmd.SetArgs([]string{"inbox", "promote", ids[1], "--as", "thread"})
	if err := promoteCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	discardCmd := newRootCmd()
	discardCmd.SetArgs([]string{"inbox", "promote", ids[2], "--as", "discard"})
	if err := discardCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	listJSON := func(t *testing.T, args ...string) []map[string]any {
		t.Helper()
		cmd := newRootCmd()
		cmd.SetArgs(append([]string{"inbox", "list"}, args...))
		var out bytes.Buffer
		cmd.SetOut(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("list %v: %v", args, err)
		}
		var env struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &env); err != nil {
			t.Fatalf("unmarshal %q: %v", out.String(), err)
		}
		return env.Items
	}

	if got := listJSON(t, "--json"); len(got) != 1 || got[0]["status"] != "raw" {
		t.Errorf("default list = %+v, want one raw entry", got)
	}
	if got := listJSON(t, "--all", "--json"); len(got) != 3 {
		t.Errorf("--all list len = %d, want 3", len(got))
	}
	if got := listJSON(t, "--status", "promoted", "--json"); len(got) != 1 || got[0]["status"] != "promoted" {
		t.Errorf("--status promoted = %+v", got)
	}
}

func TestInboxList_LimitAndSince(t *testing.T) {
	newTestVaultWithDatedInbox(t, []string{"2026-04-30", "2026-05-02", "2026-05-04"})
	cmd := newRootCmd()
	out, _, _ := runCmd(t, cmd, "inbox", "list", "--since", "2026-05-01", "--all", "--json")
	env := unmarshalListEnvelope(t, out)
	if env.Total != 2 {
		t.Errorf("total=%d want 2", env.Total)
	}
}

func TestInboxList_DefaultStatusRawStillApplies(t *testing.T) {
	newTestVaultWithMixedInbox(t)
	cmd := newRootCmd()
	out, _, _ := runCmd(t, cmd, "inbox", "list", "--json")
	env := unmarshalListEnvelope(t, out)
	if env.Total != 1 {
		t.Errorf("default should filter to status:raw; got total=%d", env.Total)
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
