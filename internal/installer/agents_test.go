package installer

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"gopkg.in/yaml.v3"

	"github.com/chonalchendo/anvil/anvil/agents"
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
	if err := os.MkdirAll(target, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	dst := filepath.Join(target, "anvil-issue-worker.md")
	if err := os.WriteFile(dst, []byte("user-edited\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
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
	if err := os.WriteFile(foreign, []byte("mine\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
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

func fakePiAgentsFS() fstest.MapFS {
	return fstest.MapFS{
		"anvil-issue-worker.md": {Data: []byte("---\nname: anvil-issue-worker\ndescription: does the thing\nmodel: sonnet\neffort: medium\ntools: Bash, Read, Glob, ToolSearch, TaskOutput, TaskStop\nskills: completing-issue\n---\nbody\n")},
		"README":                {Data: []byte("not an agent\n")},
	}
}

// piFrontmatter strictly YAML-parses the frontmatter block of an emitted pi
// agent markdown document, so assertions on translated keys can't false-pass
// against text that merely contains the right substring.
func piFrontmatter(t *testing.T, md string) map[string]any {
	t.Helper()
	parts := strings.SplitN(md, "---\n", 3)
	if len(parts) < 3 {
		t.Fatalf("emitted agent missing frontmatter delimiters\n---\n%s", md)
	}
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(parts[1]), &doc); err != nil {
		t.Fatalf("emitted frontmatter is not valid strict YAML: %v\n---\n%s", err, parts[1])
	}
	return doc
}

func TestInstallPiAgents_TranslatesModelEffortToolsAndSkills(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agents")

	changed, err := InstallPiAgents(fakePiAgentsFS(), target, false)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !changed {
		t.Error("first install should report changed=true")
	}

	b, err := os.ReadFile(filepath.Join(target, "anvil-issue-worker.md")) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil {
		t.Fatalf("read emitted agent: %v", err)
	}
	doc := piFrontmatter(t, string(b))

	if doc["name"] != "anvil-issue-worker" {
		t.Errorf("name = %v, want anvil-issue-worker", doc["name"])
	}
	if doc["description"] != "does the thing" {
		t.Errorf("description = %v, want %q", doc["description"], "does the thing")
	}
	if doc["model"] != "claude-bridge/claude-sonnet-5" {
		t.Errorf("model = %v, want claude-bridge/claude-sonnet-5", doc["model"])
	}
	if doc["thinking"] != "medium" {
		t.Errorf("thinking = %v, want medium (translated from effort)", doc["thinking"])
	}
	if doc["tools"] != "bash, read, find" {
		t.Errorf("tools = %v, want %q (Glob translated to find, Claude-only names dropped)", doc["tools"], "bash, read, find")
	}
	if doc["skills"] != "completing-issue" {
		t.Errorf("skills = %v, want completing-issue", doc["skills"])
	}
	if _, ok := doc["effort"]; ok {
		t.Errorf("effort key should not be emitted, got %v", doc["effort"])
	}
	if _, err := os.Stat(filepath.Join(target, "README")); !os.IsNotExist(err) {
		t.Errorf("non-.md entry should be skipped, got err=%v", err)
	}
}

func TestInstallPiAgents_QuotesDescriptionWithColonSpace(t *testing.T) {
	srcFS := fstest.MapFS{
		"agent.md": {Data: []byte("---\nname: agent\ndescription: Use when X: does the thing\nmodel: sonnet\n---\nbody\n")},
	}
	target := filepath.Join(t.TempDir(), "agents")

	if _, err := InstallPiAgents(srcFS, target, false); err != nil {
		t.Fatalf("install: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(target, "agent.md")) //nolint:gosec // path is test-controlled or application-managed; not user input
	if err != nil {
		t.Fatalf("read emitted agent: %v", err)
	}

	var doc map[string]any
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "description: ") {
			if err := yaml.Unmarshal([]byte(line), &doc); err != nil {
				t.Fatalf("description line is not valid strict YAML: %v\nline=%q", err, line)
			}
		}
	}
	if got := doc["description"]; got != "Use when X: does the thing" {
		t.Errorf("description round-tripped through YAML as %q", got)
	}
}

func TestInstallPiAgents_UnmappedModelIsError(t *testing.T) {
	srcFS := fstest.MapFS{
		"agent.md": {Data: []byte("---\nname: agent\ndescription: d\nmodel: gpt-5\n---\nbody\n")},
	}
	if _, err := InstallPiAgents(srcFS, filepath.Join(t.TempDir(), "agents"), false); err == nil {
		t.Fatal("expected error for a model alias with no pi translation")
	}
}

func TestInstallPiAgents_IdempotentNoOp(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agents")
	if _, err := InstallPiAgents(fakePiAgentsFS(), target, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	changed, err := InstallPiAgents(fakePiAgentsFS(), target, false)
	if err != nil {
		t.Fatalf("second install: %v", err)
	}
	if changed {
		t.Error("re-installing identical content should report changed=false")
	}
}

func TestInstallPiAgents_RefusesDivergentWithoutForce(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(target, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	dst := filepath.Join(target, "anvil-issue-worker.md")
	if err := os.WriteFile(dst, []byte("user-edited\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	if _, err := InstallPiAgents(fakePiAgentsFS(), target, false); err == nil {
		t.Fatal("expected refusal overwriting a divergent file without --force")
	}

	changed, err := InstallPiAgents(fakePiAgentsFS(), target, true)
	if err != nil {
		t.Fatalf("force install: %v", err)
	}
	if !changed {
		t.Error("force install over divergent file should report changed=true")
	}
}

func TestRemovePiAgents_LeavesForeignContent(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agents")
	if _, err := InstallPiAgents(fakePiAgentsFS(), target, false); err != nil {
		t.Fatalf("install: %v", err)
	}
	foreign := filepath.Join(target, "user-agent.md")
	if err := os.WriteFile(foreign, []byte("mine\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	changed, err := RemovePiAgents(fakePiAgentsFS(), target)
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

func TestRemoveAgents_LeavesDivergentSameName(t *testing.T) {
	target := filepath.Join(t.TempDir(), "agents")
	if err := os.MkdirAll(target, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		t.Fatal(err)
	}
	// A user-edited file sharing an anvil agent's name must survive removal —
	// RemoveAgents only deletes content that still matches the embedded copy.
	dst := filepath.Join(target, "anvil-issue-worker.md")
	if err := os.WriteFile(dst, []byte("user-customised\n"), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
		t.Fatal(err)
	}

	changed, err := RemoveAgents(fakeAgentsFS(), target)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if changed {
		t.Error("removing only a divergent same-name file should report changed=false")
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("divergent same-name file should survive removal: %v", err)
	}
}

// Binds the real embedded bundle: if someone adds a bundled agent whose model
// alias is absent from claudeModelToPiRef (or whose frontmatter otherwise
// fails translation), `anvil install agents --target pi|codex` fails at the
// user's install — synthetic MapFS fixtures cannot catch that.
func TestBundledAgents_TranslateForAllTargets(t *testing.T) {
	names, err := listAgentFiles(agents.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("embedded agents bundle is empty")
	}
	for _, name := range names {
		src, err := fs.ReadFile(agents.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := piAgentMarkdown(src); err != nil {
			t.Errorf("piAgentMarkdown(%s): %v", name, err)
		}
		if _, err := codexAgentTOML(src); err != nil {
			t.Errorf("codexAgentTOML(%s): %v", name, err)
		}
	}
}
