package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// installTranslatedAgents writes translate(src) for each embedded agent to
// target/<name>.md. A byte-identical file is a no-op; a divergent one is
// refused unless force is true. Shared by the markdown-emitting targets so
// the clobber contract lives in one place.
func installTranslatedAgents(srcFS fs.FS, target, targetName string, force bool, translate func([]byte) (string, error)) (bool, error) {
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
		want, err := translate(src)
		if err != nil {
			return false, fmt.Errorf("translate agent %s: %w", name, err)
		}
		dst := filepath.Join(target, name)
		got, err := os.ReadFile(dst) //nolint:gosec // path is test-controlled or application-managed; not user input
		switch {
		case err == nil && string(got) == want:
			continue
		case err == nil && !force:
			return false, fmt.Errorf("refusing to overwrite non-matching %s; run `anvil install agents --target %s --force` to redeploy", dst, targetName)
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

// removeTranslatedAgents deletes target/<name>.md for each embedded agent
// whose on-disk content still matches translate(src); divergent or foreign
// files are left untouched.
func removeTranslatedAgents(srcFS fs.FS, target string, translate func([]byte) (string, error)) (bool, error) {
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
		want, err := translate(src)
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

// claudeModelToOpenCodeRef translates anvil's Claude Code model aliases to
// OpenCode's provider/model-id form under the built-in anthropic provider.
// An alias with no entry makes the emit fail rather than hand OpenCode a ref
// it cannot resolve.
var claudeModelToOpenCodeRef = map[string]string{
	"sonnet": "anthropic/claude-sonnet-5",
	"opus":   "anthropic/claude-opus-5",
	"haiku":  "anthropic/claude-haiku-4-5",
}

// claudeToolToOpenCodeTool maps Claude Code built-in tool names to OpenCode's
// lowercase tool keys. Claude-only tools (ToolSearch, TaskOutput, TaskStop)
// have no entry and are dropped.
var claudeToolToOpenCodeTool = map[string]string{
	"bash":      "bash",
	"read":      "read",
	"edit":      "edit",
	"write":     "write",
	"grep":      "grep",
	"glob":      "glob",
	"webfetch":  "webfetch",
	"websearch": "websearch",
}

// InstallOpenCodeAgents emits each embedded agent as OpenCode agent markdown
// at target/<name>.md (OpenCode derives the agent name from the filename).
func InstallOpenCodeAgents(srcFS fs.FS, target string, force bool) (bool, error) {
	return installTranslatedAgents(srcFS, target, "opencode", force, openCodeAgentMarkdown)
}

// RemoveOpenCodeAgents deletes emitted copies that still match the translation.
func RemoveOpenCodeAgents(srcFS fs.FS, target string) (bool, error) {
	return removeTranslatedAgents(srcFS, target, openCodeAgentMarkdown)
}

// openCodeAgentMarkdown translates one embedded agent into OpenCode agent
// markdown: description (double-quoted, since descriptions contain `: `),
// mode: subagent, the translated model, and a tools enable-map for the
// mappable subset. name/effort/skills have no OpenCode frontmatter key and are
// dropped; the body is the system prompt and copies through.
func openCodeAgentMarkdown(md []byte) (string, error) {
	fields, body, err := parseAgentMarkdown(md)
	if err != nil {
		return "", err
	}
	if fields["name"] == "" || fields["description"] == "" {
		return "", errors.New("agent frontmatter missing name or description")
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %q\n", fields["description"])
	b.WriteString("mode: subagent\n")
	if model := fields["model"]; model != "" {
		ref, ok := claudeModelToOpenCodeRef[model]
		if !ok {
			return "", fmt.Errorf("model %q has no opencode ref translation", model)
		}
		fmt.Fprintf(&b, "model: %s\n", ref)
	}
	var tools []string
	for _, t := range strings.Split(fields["tools"], ",") {
		if oc, ok := claudeToolToOpenCodeTool[strings.ToLower(strings.TrimSpace(t))]; ok {
			tools = append(tools, oc)
		}
	}
	if len(tools) > 0 {
		b.WriteString("tools:\n")
		for _, t := range tools {
			fmt.Fprintf(&b, "  %s: true\n", t)
		}
	}
	b.WriteString("---\n")
	b.WriteString(body)
	return b.String(), nil
}
