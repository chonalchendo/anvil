package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// claudeModelToAnteRef translates anvil's Claude Code model aliases to the
// canonical Ante catalog id (docs.antigma.ai/reference/catalog-reference).
// Ante does not resolve a bare Claude alias ("sonnet", "opus", "haiku"), so
// every alias the embedded bundle uses must have an entry here or
// InstallAnteAgents refuses to emit — mirrors claudeModelToPiRef.
var claudeModelToAnteRef = map[string]string{
	"sonnet": "claude-sonnet-5",
	"opus":   "claude-opus-5",
	"haiku":  "claude-haiku-4-5",
}

// claudeToolToAnteTool translates anvil's Claude Code built-in tool names to
// Ante's built-in tool names (docs.antigma.ai/extend/subagents). Ante's
// built-ins are near-identical to Claude Code's own naming, so this map is
// mostly identity. A Claude-only tool with no Ante equivalent (ToolSearch,
// TaskOutput, TaskStop — Ante's Bash tool already backgrounds long-running
// commands natively, needing no separate drain-tool pair) has no entry and is
// dropped by anteToolNames rather than passed through.
var claudeToolToAnteTool = map[string]string{
	"bash":      "Bash",
	"read":      "Read",
	"edit":      "Edit",
	"write":     "Write",
	"grep":      "Grep",
	"glob":      "Glob",
	"websearch": "WebSearch",
	"webfetch":  "WebFetch",
}

// anteToolNames translates a Claude Code agent's comma-separated tools list
// into the Ante-resolvable subset: each entry is matched case-insensitively
// against claudeToolToAnteTool and emitted in Ante's tool-name casing;
// anything unmatched has no Ante-side meaning and is dropped.
func anteToolNames(claudeTools string) []string {
	var kept []string
	for _, t := range strings.Split(claudeTools, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if ante, ok := claudeToolToAnteTool[strings.ToLower(t)]; ok {
			kept = append(kept, ante)
		}
	}
	return kept
}

// InstallAnteAgents translates each embedded *.md agent into an
// Ante-compatible subagent markdown file at target/<name>.md. Ante's
// sub-agent discovery reads the same name/description/model/tools
// frontmatter shape Claude Code uses (docs.antigma.ai/extend/subagents) —
// anteAgentMarkdown carries name/description/model through and narrows tools;
// the body copies through unchanged. Mirrors InstallPiAgents' clobber
// contract: a byte-identical file is a no-op, a divergent one is refused
// unless force is true.
func InstallAnteAgents(srcFS fs.FS, target string, force bool) (bool, error) {
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
		want, err := anteAgentMarkdown(src)
		if err != nil {
			return false, fmt.Errorf("translate agent %s: %w", name, err)
		}
		dst := filepath.Join(target, name)
		got, err := os.ReadFile(dst) //nolint:gosec // path is test-controlled or application-managed; not user input
		switch {
		case err == nil && string(got) == want:
			continue
		case err == nil && !force:
			return false, fmt.Errorf("refusing to overwrite non-matching %s; run `anvil install agents --target ante --force` to redeploy", dst)
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

// RemoveAnteAgents deletes target/<name>.md for each embedded agent whose
// on-disk content still matches the translated copy. Divergent or foreign
// files are left untouched, mirroring RemovePiAgents.
func RemoveAnteAgents(srcFS fs.FS, target string) (bool, error) {
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
		want, err := anteAgentMarkdown(src)
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

// anteAgentMarkdown translates one embedded agent markdown file into an
// Ante-compatible subagent markdown document (docs.antigma.ai/extend/subagents):
// name carries through as a plain scalar, description as a double-quoted YAML
// scalar (anvil's descriptions routinely contain `: `, which breaks an
// unquoted YAML scalar — see piAgentMarkdown), model is translated via
// claudeModelToAnteRef to the canonical catalog id Ante's model resolution
// requires (an untranslatable alias is a hard error, same as
// piAgentMarkdown — emitting it would hand Ante a ref it can't resolve),
// tools narrows to the Ante-resolvable subset via anteToolNames as a YAML
// block list (the shape Ante's own docs show), and effort/skills are dropped
// outright — Ante's frontmatter has no equivalent keys for either. The body
// is Ante's system prompt and copies through unchanged.
func anteAgentMarkdown(md []byte) (string, error) {
	fields, body, err := parseAgentMarkdown(md)
	if err != nil {
		return "", err
	}
	if fields["name"] == "" || fields["description"] == "" {
		return "", errors.New("agent frontmatter missing name or description")
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", fields["name"])
	fmt.Fprintf(&b, "description: %q\n", fields["description"])
	if model := fields["model"]; model != "" {
		anteModel, ok := claudeModelToAnteRef[model]
		if !ok {
			return "", fmt.Errorf("model %q has no ante ref translation", model)
		}
		fmt.Fprintf(&b, "model: %s\n", anteModel)
	}
	if tools := anteToolNames(fields["tools"]); len(tools) > 0 {
		b.WriteString("tools:\n")
		for _, t := range tools {
			fmt.Fprintf(&b, "  - %s\n", t)
		}
	}
	b.WriteString("---\n")
	b.WriteString(body)
	return b.String(), nil
}
