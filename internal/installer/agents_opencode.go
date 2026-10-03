package installer

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// openCodeTools is the set of Claude Code built-in tool names (lowercased)
// that OpenCode also exposes under the same key. Claude-only tools
// (ToolSearch, TaskOutput, TaskStop) are absent and dropped.
var openCodeTools = map[string]bool{
	"bash": true, "read": true, "edit": true, "write": true, "grep": true,
	"glob": true, "webfetch": true, "websearch": true,
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
// mode: subagent, and a deny-by-default tools map enabling the mappable
// subset (plus skill when skills: is declared). model is omitted so
// subagents inherit the session model — tier selection is user config.
// name/effort/skills have no OpenCode frontmatter key and are dropped; the
// body is the system prompt and copies through. OpenCode folds write into
// edit, so Edit-only agents gain write.
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
	// Deny-by-default: OpenCode leaves every tool enabled unless "*" is
	// switched off, so a bare enable-list would not restrict anything.
	b.WriteString("tools:\n")
	b.WriteString("  \"*\": false\n")
	for _, t := range strings.Split(fields["tools"], ",") {
		if name := strings.ToLower(strings.TrimSpace(t)); openCodeTools[name] {
			fmt.Fprintf(&b, "  %s: true\n", name)
		}
	}
	if fields["skills"] != "" {
		b.WriteString("  skill: true\n")
	}
	b.WriteString("---\n")
	b.WriteString(body)
	return b.String(), nil
}
