package installer

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// InstallAgents copies each top-level *.md agent in srcFS to target/<name>.md.
// target is the user-agents dir (typically ~/.claude/agents), shared with
// non-anvil agent files — so it writes per-file rather than mirroring the tree
// the way InstallSkills does. Agents are single files with no symlink/refresh
// machinery; idempotent re-copy is the whole story.
//
// An existing file whose content already matches the embedded copy is a no-op;
// one that diverges is refused unless force is true, mirroring InstallSkills's
// promise not to clobber user-owned content silently.
func InstallAgents(srcFS fs.FS, target string, force bool) (bool, error) {
	names, err := listAgentFiles(srcFS)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(target, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		return false, fmt.Errorf("mkdir target %s: %w", target, err)
	}
	changed := false
	for _, name := range names {
		want, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return false, fmt.Errorf("read embedded agent %s: %w", name, err)
		}
		dst := filepath.Join(target, name)
		got, err := os.ReadFile(dst) //nolint:gosec // path is test-controlled or application-managed; not user input
		switch {
		case err == nil && bytes.Equal(got, want):
			continue
		case err == nil && !force:
			return false, fmt.Errorf("refusing to overwrite non-matching %s; run `anvil install agents --force` to redeploy", dst)
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return false, fmt.Errorf("read %s: %w", dst, err)
		}
		if err := os.WriteFile(dst, want, 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
			return false, fmt.Errorf("write %s: %w", dst, err)
		}
		changed = true
	}
	return changed, nil
}

// RemoveAgents deletes target/<name>.md for each embedded agent whose on-disk
// content still matches the embedded copy (anvil-owned). Divergent or foreign
// files are left untouched, mirroring RemoveSkills.
func RemoveAgents(srcFS fs.FS, target string) (bool, error) {
	names, err := listAgentFiles(srcFS)
	if err != nil {
		return false, err
	}
	changed := false
	for _, name := range names {
		want, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return false, fmt.Errorf("read embedded agent %s: %w", name, err)
		}
		dst := filepath.Join(target, name)
		got, err := os.ReadFile(dst) //nolint:gosec // path is test-controlled or application-managed; not user input
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false, fmt.Errorf("read %s: %w", dst, err)
		}
		if !bytes.Equal(got, want) {
			continue
		}
		if err := os.Remove(dst); err != nil {
			return false, fmt.Errorf("remove %s: %w", dst, err)
		}
		changed = true
	}
	return changed, nil
}

// InstallCodexAgents translates each embedded *.md agent into a Codex
// custom-agent TOML file at target/<name>.toml. Mirrors InstallAgents' clobber
// contract: a byte-identical file is a no-op, a divergent one is refused unless
// force is true.
func InstallCodexAgents(srcFS fs.FS, target string, force bool) (bool, error) {
	names, err := listAgentFiles(srcFS)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(target, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		return false, fmt.Errorf("mkdir target %s: %w", target, err)
	}
	changed := false
	for _, name := range names {
		src, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return false, fmt.Errorf("read embedded agent %s: %w", name, err)
		}
		want, err := codexAgentTOML(src)
		if err != nil {
			return false, fmt.Errorf("translate agent %s: %w", name, err)
		}
		dst := filepath.Join(target, strings.TrimSuffix(name, ".md")+".toml")
		got, err := os.ReadFile(dst) //nolint:gosec // path is test-controlled or application-managed; not user input
		switch {
		case err == nil && string(got) == want:
			continue
		case err == nil && !force:
			return false, fmt.Errorf("refusing to overwrite non-matching %s; run `anvil install agents --target codex --force` to redeploy", dst)
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return false, fmt.Errorf("read %s: %w", dst, err)
		}
		if err := os.WriteFile(dst, []byte(want), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
			return false, fmt.Errorf("write %s: %w", dst, err)
		}
		changed = true
	}
	return changed, nil
}

// RemoveCodexAgents deletes target/<name>.toml for each embedded agent whose
// on-disk content still matches the translated copy. Divergent or foreign files
// are left untouched, mirroring RemoveAgents.
func RemoveCodexAgents(srcFS fs.FS, target string) (bool, error) {
	names, err := listAgentFiles(srcFS)
	if err != nil {
		return false, err
	}
	changed := false
	for _, name := range names {
		src, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return false, fmt.Errorf("read embedded agent %s: %w", name, err)
		}
		want, err := codexAgentTOML(src)
		if err != nil {
			return false, fmt.Errorf("translate agent %s: %w", name, err)
		}
		dst := filepath.Join(target, strings.TrimSuffix(name, ".md")+".toml")
		got, err := os.ReadFile(dst) //nolint:gosec // path is test-controlled or application-managed; not user input
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false, fmt.Errorf("read %s: %w", dst, err)
		}
		if string(got) != want {
			continue
		}
		if err := os.Remove(dst); err != nil {
			return false, fmt.Errorf("remove %s: %w", dst, err)
		}
		changed = true
	}
	return changed, nil
}

// codexAgentTOML translates one embedded agent markdown file into a Codex
// custom-agent TOML document. Codex agents are a different schema from anvil's
// Claude markdown subagent (developers.openai.com/codex/subagents): the three
// required keys are name, description, developer_instructions. Frontmatter
// name/description map to the first two, the body becomes developer_instructions,
// and effort (low/medium/high) maps to model_reasoning_effort. The frontmatter
// model/tools/skills are intentionally dropped — model is a Claude model id
// Codex can't resolve and tools/skills use Claude-specific names, so emitting
// them would yield TOML Codex rejects at load. Translating only the
// cleanly-mapping subset keeps the artifact valid.
func codexAgentTOML(md []byte) (string, error) {
	fields, body, err := parseAgentMarkdown(md)
	if err != nil {
		return "", err
	}
	if fields["name"] == "" || fields["description"] == "" {
		return "", errors.New("agent frontmatter missing name or description")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "name = %s\n", tomlBasicString(fields["name"]))
	fmt.Fprintf(&b, "description = %s\n", tomlBasicString(fields["description"]))
	switch fields["effort"] {
	case "low", "medium", "high":
		fmt.Fprintf(&b, "model_reasoning_effort = %s\n", tomlBasicString(fields["effort"]))
	}
	fmt.Fprintf(&b, "developer_instructions = %s\n", tomlMultilineString(strings.TrimSpace(body)))
	return b.String(), nil
}

// parseAgentMarkdown splits an agent markdown file into its frontmatter fields
// and body. Agent files follow the Claude Code subagent convention: a `---`
// fenced header of `key: value` lines whose value runs to end-of-line (so a
// `: ` inside a description is literal), then the markdown body. That
// value-to-EOL rule is why this is a line parser, not a strict YAML decode —
// the shipped descriptions contain `: ` that a YAML scalar would reject.
func parseAgentMarkdown(md []byte) (map[string]string, string, error) {
	s := string(md)
	const delim = "---\n"
	if !strings.HasPrefix(s, delim) {
		return nil, "", errors.New("agent markdown missing frontmatter")
	}
	rest := s[len(delim):]
	end := strings.Index(rest, "\n"+delim)
	if end < 0 {
		return nil, "", errors.New("agent markdown missing closing frontmatter delimiter")
	}
	header, body := rest[:end], rest[end+len("\n"+delim):]
	fields := map[string]string{}
	for _, line := range strings.Split(header, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return fields, body, nil
}

// Both string encoders below assume their input is anvil-authored, committed
// agent markdown — clean UTF-8 text whose only TOML metacharacters are the
// backslash and double-quote. They escape those; C0 control characters, which
// TOML forbids in basic strings, cannot occur in the embedded bundle (an
// invariant, not a runtime check). A live `tomllib` parse in the install
// verification would catch a violation if a future agent broke it.

// tomlBasicString quotes s as a single-line TOML basic string.
func tomlBasicString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`)
	return `"` + r.Replace(s) + `"`
}

// tomlMultilineString quotes s as a TOML multi-line basic string. Newlines are
// kept literal; escaping every double-quote (rather than only runs of three)
// keeps the body from reconstituting the `"""` delimiter.
func tomlMultilineString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return "\"\"\"\n" + r.Replace(s) + "\n\"\"\""
}

// claudeModelToPiRef translates anvil's Claude Code model aliases to a
// pi-resolvable model ref: provider/modelId under the pi-claude-bridge
// provider (developers.pi.dev/coding-agent/model-refs). Pi cannot resolve a
// bare Claude alias ("sonnet", "opus", "haiku"), so every alias the embedded
// bundle uses must have an entry here or InstallPiAgents refuses to emit.
var claudeModelToPiRef = map[string]string{
	"sonnet": "claude-bridge/claude-sonnet-5",
	"opus":   "claude-bridge/claude-opus-5",
	"haiku":  "claude-bridge/claude-haiku-4-5",
}

// claudeToolToPiTool translates anvil's Claude Code built-in tool names to
// pi-subagents' built-in tool names (developers.pi.dev/coding-agent, the
// @tintinweb/pi-subagents schema). A Claude-only tool (ToolSearch, TaskOutput,
// TaskStop, WebSearch, WebFetch — pi has no equivalent) has no entry and is
// dropped by piToolNames rather than passed through, since pi's runtime
// cannot resolve it. Glob maps to pi's `find` builtin ("Search for files by
// glob pattern") — a direct equivalent, not a drop.
var claudeToolToPiTool = map[string]string{
	"bash":  "bash",
	"read":  "read",
	"edit":  "edit",
	"write": "write",
	"grep":  "grep",
	"find":  "find",
	"glob":  "find",
	"ls":    "ls",
}

// piToolNames translates a Claude Code agent's comma-separated tools list
// into the pi-resolvable subset: each entry is matched case-insensitively
// against claudeToolToPiTool and emitted as pi's lowercase name, or passed
// through unchanged if already an `ext:`-selector (pi-subagents-native, not
// a Claude name); anything else has no pi-side meaning and is dropped.
func piToolNames(claudeTools string) string {
	var kept []string
	for _, t := range strings.Split(claudeTools, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "ext:") {
			kept = append(kept, t)
			continue
		}
		if pi, ok := claudeToolToPiTool[strings.ToLower(t)]; ok {
			kept = append(kept, pi)
		}
	}
	return strings.Join(kept, ", ")
}

// InstallPiAgents translates each embedded *.md agent into a pi-compatible
// subagent markdown file at target/<name>.md. Pi's subagent extension
// discovers the same name/description/tools/model frontmatter shape Claude
// Code uses, plus skills (a preload list) and thinking (an effort level) —
// piAgentMarkdown carries the model alias, tools list, skills, and effort
// through their respective translations; the body copies through unchanged.
// Mirrors InstallCodexAgents' clobber contract: a byte-identical file is a
// no-op, a divergent one is refused unless force is true.
func InstallPiAgents(srcFS fs.FS, target string, force bool) (bool, error) {
	names, err := listAgentFiles(srcFS)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(target, 0o755); err != nil { //nolint:gosec // 0755 is correct for directories that must be traversable
		return false, fmt.Errorf("mkdir target %s: %w", target, err)
	}
	changed := false
	for _, name := range names {
		src, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return false, fmt.Errorf("read embedded agent %s: %w", name, err)
		}
		want, err := piAgentMarkdown(src)
		if err != nil {
			return false, fmt.Errorf("translate agent %s: %w", name, err)
		}
		dst := filepath.Join(target, name)
		got, err := os.ReadFile(dst) //nolint:gosec // path is test-controlled or application-managed; not user input
		switch {
		case err == nil && string(got) == want:
			continue
		case err == nil && !force:
			return false, fmt.Errorf("refusing to overwrite non-matching %s; run `anvil install agents --target pi --force` to redeploy", dst)
		case err != nil && !errors.Is(err, os.ErrNotExist):
			return false, fmt.Errorf("read %s: %w", dst, err)
		}
		if err := os.WriteFile(dst, []byte(want), 0o644); err != nil { //nolint:gosec // 0644 is correct for config/data files readable by owner and group
			return false, fmt.Errorf("write %s: %w", dst, err)
		}
		changed = true
	}
	return changed, nil
}

// RemovePiAgents deletes target/<name>.md for each embedded agent whose
// on-disk content still matches the translated copy. Divergent or foreign
// files are left untouched, mirroring RemoveCodexAgents.
func RemovePiAgents(srcFS fs.FS, target string) (bool, error) {
	names, err := listAgentFiles(srcFS)
	if err != nil {
		return false, err
	}
	changed := false
	for _, name := range names {
		src, err := fs.ReadFile(srcFS, name)
		if err != nil {
			return false, fmt.Errorf("read embedded agent %s: %w", name, err)
		}
		want, err := piAgentMarkdown(src)
		if err != nil {
			return false, fmt.Errorf("translate agent %s: %w", name, err)
		}
		dst := filepath.Join(target, name)
		got, err := os.ReadFile(dst) //nolint:gosec // path is test-controlled or application-managed; not user input
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return false, fmt.Errorf("read %s: %w", dst, err)
		}
		if string(got) != want {
			continue
		}
		if err := os.Remove(dst); err != nil {
			return false, fmt.Errorf("remove %s: %w", dst, err)
		}
		changed = true
	}
	return changed, nil
}

// piAgentMarkdown translates one embedded agent markdown file into a
// pi-compatible subagent markdown document: name carries through as a plain
// scalar, description as a double-quoted YAML scalar (anvil's descriptions
// routinely contain `: `, which pi's strict YAML frontmatter parse rejects
// unquoted — and pi's subagent extension calls that parse uncaught, so one
// malformed file aborts discovery of every user agent), model is translated
// via claudeModelToPiRef (an untranslatable alias is a hard error — emitting
// it would hand pi a ref it can't resolve), tools is narrowed to the
// pi-resolvable subset via piToolNames, effort carries through as pi's
// `thinking:` key (same low/medium/high/etc scale, different key name), and
// skills carries through unchanged as pi's preload list (pi-subagents reads
// the same comma-separated shape anvil already authors). The body is pi's
// system prompt and is not itself Claude-specific, so it copies through
// unchanged.
func piAgentMarkdown(md []byte) (string, error) {
	fields, body, err := parseAgentMarkdown(md)
	if err != nil {
		return "", err
	}
	if fields["name"] == "" || fields["description"] == "" {
		return "", errors.New("agent frontmatter missing name or description")
	}
	piModel, ok := claudeModelToPiRef[fields["model"]]
	if !ok {
		return "", fmt.Errorf("model %q has no pi ref translation", fields["model"])
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", fields["name"])
	fmt.Fprintf(&b, "description: %q\n", fields["description"])
	fmt.Fprintf(&b, "model: %s\n", piModel)
	if effort := fields["effort"]; effort != "" {
		fmt.Fprintf(&b, "thinking: %s\n", effort)
	}
	if tools := piToolNames(fields["tools"]); tools != "" {
		fmt.Fprintf(&b, "tools: %s\n", tools)
	}
	if skills := fields["skills"]; skills != "" {
		fmt.Fprintf(&b, "skills: %s\n", skills)
	}
	b.WriteString("---\n")
	b.WriteString(body)
	return b.String(), nil
}

func listAgentFiles(srcFS fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(srcFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read agents FS: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}
