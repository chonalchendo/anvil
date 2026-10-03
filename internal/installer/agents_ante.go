package installer

import (
	"errors"
	"fmt"
	"io/fs"
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
// the body copies through unchanged. Clobber contract: see installTranslatedAgents.
func InstallAnteAgents(srcFS fs.FS, target string, force bool) (bool, error) {
	return installTranslatedAgents(srcFS, target, "ante", force, anteAgentMarkdown)
}

// RemoveAnteAgents deletes target/<name>.md for each embedded agent whose
// on-disk content still matches the translated copy. Divergent or foreign
// files are left untouched.
func RemoveAnteAgents(srcFS fs.FS, target string) (bool, error) {
	return removeTranslatedAgents(srcFS, target, anteAgentMarkdown)
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
