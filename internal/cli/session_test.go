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
	"github.com/chonalchendo/anvil/internal/state"
)

const fakeUUID = "01234567-89ab-cdef-0123-456789abcdef"

func setupSessionEnv(t *testing.T) string {
	t.Helper()
	vault := setupVault(t)
	t.Setenv("ANVIL_STATE_DIR", t.TempDir())
	return vault
}

func TestSession_Emit_NoActiveThread(t *testing.T) {
	vault := setupSessionEnv(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "emit", "--session-id", fakeUUID})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("emit: %v", err)
	}

	path := filepath.Join(vault, "10-sessions", fakeUUID+".md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatalf("load artifact: %v", err)
	}
	if a.FrontMatter["session_id"] != fakeUUID {
		t.Errorf("session_id = %v, want %q", a.FrontMatter["session_id"], fakeUUID)
	}
	if a.FrontMatter["status"] != "raw" {
		t.Errorf("status = %v, want %q", a.FrontMatter["status"], "raw")
	}
	if a.FrontMatter["source"] != "claude-code" {
		t.Errorf("source = %v, want %q", a.FrontMatter["source"], "claude-code")
	}
	related, _ := a.FrontMatter["related"].([]any)
	if len(related) != 0 {
		t.Errorf("related = %v, want empty", related)
	}
	if err := schema.Validate("session", a.FrontMatter); err != nil {
		t.Errorf("frontmatter fails schema: %v", err)
	}
}

func TestSession_Emit_StampsActiveThread(t *testing.T) {
	vault := setupSessionEnv(t)
	if err := state.WriteActiveThread("research-ducklake"); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "emit", "--session-id", fakeUUID})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("emit: %v", err)
	}

	path := filepath.Join(vault, "10-sessions", fakeUUID+".md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	related, ok := a.FrontMatter["related"].([]any)
	if !ok || len(related) != 1 {
		t.Fatalf("related = %v, want one entry", a.FrontMatter["related"])
	}
	if related[0] != "[[thread.research-ducklake]]" {
		t.Errorf("related[0] = %v, want %q", related[0], "[[thread.research-ducklake]]")
	}
}

func TestSession_Emit_Idempotent(t *testing.T) {
	vault := setupSessionEnv(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "emit", "--session-id", fakeUUID})
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("first emit: %v", err)
	}
	path := filepath.Join(vault, "10-sessions", fakeUUID+".md")
	content1, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := state.WriteActiveThread("research-ducklake"); err != nil {
		t.Fatal(err)
	}
	cmd2 := newRootCmd()
	cmd2.SetArgs([]string{"session", "emit", "--session-id", fakeUUID})
	cmd2.SetOut(&bytes.Buffer{})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("second emit: %v", err)
	}
	content2, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content1) != string(content2) {
		t.Errorf("file rewritten on second emit (contents changed)")
	}
}

func TestSession_Emit_JSON(t *testing.T) {
	vault := setupSessionEnv(t)
	if err := state.WriteActiveThread("research-ducklake"); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "emit", "--session-id", fakeUUID, "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("emit: %v", err)
	}

	var got struct {
		ID      string   `json:"id"`
		Path    string   `json:"path"`
		Related []string `json:"related"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse json: %v\noutput: %s", err, out.String())
	}
	if got.ID != fakeUUID {
		t.Errorf("id = %q", got.ID)
	}
	wantPath := filepath.Join(vault, "10-sessions", fakeUUID+".md")
	if got.Path != wantPath {
		t.Errorf("path = %q, want %q", got.Path, wantPath)
	}
	if len(got.Related) != 1 || got.Related[0] != "[[thread.research-ducklake]]" {
		t.Errorf("related = %v", got.Related)
	}
}

func TestSession_Emit_RejectsEmptySessionID(t *testing.T) {
	setupSessionEnv(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "emit", "--session-id", ""})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "session-id") {
		t.Errorf("err = %v, want a session-id error", err)
	}
}

func TestSession_Emit_RejectsUnknownSource(t *testing.T) {
	setupSessionEnv(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "emit", "--session-id", fakeUUID, "--source", "vscode"})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown source")
	}
}

func TestSession_Emit_FromStdin(t *testing.T) {
	vault := setupSessionEnv(t)
	if err := state.WriteActiveThread("research-ducklake"); err != nil {
		t.Fatal(err)
	}

	stdinJSON := `{"session_id":"` + fakeUUID + `","source":"startup","cwd":"/tmp","hook_event_name":"SessionStart"}`
	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "emit", "--from-stdin"})
	cmd.SetIn(strings.NewReader(stdinJSON))
	cmd.SetOut(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("emit: %v", err)
	}

	path := filepath.Join(vault, "10-sessions", fakeUUID+".md")
	a, err := core.LoadArtifact(path)
	if err != nil {
		t.Fatalf("load artifact: %v", err)
	}
	if a.FrontMatter["session_id"] != fakeUUID {
		t.Errorf("session_id = %v, want %q", a.FrontMatter["session_id"], fakeUUID)
	}
	if a.FrontMatter["source"] != "claude-code" {
		t.Errorf("source = %v, want claude-code (default mapping)", a.FrontMatter["source"])
	}
	related, _ := a.FrontMatter["related"].([]any)
	if len(related) != 1 || related[0] != "[[thread.research-ducklake]]" {
		t.Errorf("related = %v", related)
	}
}

func TestSession_Emit_RejectsBothInputModes(t *testing.T) {
	setupSessionEnv(t)

	cmd := newRootCmd()
	cmd.SetArgs([]string{"session", "emit", "--session-id", fakeUUID, "--from-stdin"})
	cmd.SetIn(strings.NewReader(`{"session_id":"x"}`))
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when both --session-id and --from-stdin given")
	}
}
