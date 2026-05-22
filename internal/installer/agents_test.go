package installer

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func fakeAgentsFS() fstest.MapFS {
	return fstest.MapFS{
		"anvil-issue-worker.md": {Data: []byte("---\nname: anvil-issue-worker\n---\nbody\n")},
		"README":                {Data: []byte("not an agent\n")},
	}
}

func TestInstallAgents_CopiesMarkdownOnly(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agents")

	changed, err := InstallAgents(fakeAgentsFS(), target, false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !changed {
		t.Error("first install should report changed=true")
	}
	if _, err := os.Stat(filepath.Join(target, "anvil-issue-worker.md")); err != nil {
		t.Errorf("agent file not deployed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "README")); !os.IsNotExist(err) {
		t.Errorf("non-.md entry should be skipped, got err=%v", err)
	}
}

func TestInstallAgents_IdempotentNoOp(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agents")
	if _, err := InstallAgents(fakeAgentsFS(), target, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	changed, err := InstallAgents(fakeAgentsFS(), target, false)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if changed {
		t.Error("re-installing identical content should report changed=false")
	}
}

func TestInstallAgents_RefusesDivergentWithoutForce(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(target, "anvil-issue-worker.md")
	if err := os.WriteFile(dst, []byte("user-edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := InstallAgents(fakeAgentsFS(), target, false); err == nil {
		t.Fatal("expected refusal overwriting a divergent file without --force")
	}

	changed, err := InstallAgents(fakeAgentsFS(), target, true)
	if err != nil {
		t.Fatalf("force install: %v", err)
	}
	if !changed {
		t.Error("force install over divergent file should report changed=true")
	}
}

func TestRemoveAgents_LeavesForeignContent(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agents")
	if _, err := InstallAgents(fakeAgentsFS(), target, false); err != nil {
		t.Fatalf("install: %v", err)
	}
	foreign := filepath.Join(target, "user-agent.md")
	if err := os.WriteFile(foreign, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := RemoveAgents(fakeAgentsFS(), target)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !changed {
		t.Error("removing a deployed anvil agent should report changed=true")
	}
	if _, err := os.Stat(filepath.Join(target, "anvil-issue-worker.md")); !os.IsNotExist(err) {
		t.Errorf("anvil agent should be removed, got err=%v", err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Errorf("foreign agent file should survive removal: %v", err)
	}
}
