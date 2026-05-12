#!/usr/bin/env bash
# PreToolUse hook: when the agent is about to Edit/Write a *.go file, inject
# the Go conventions, code-design, and agent-CLI-principles docs into context
# once per session. The hook defers all permission decisions to Claude Code's
# normal flow — it only adds context, never approves or denies.
#
# See docs/code-design.md "Define Errors Out": failure to find the docs or
# parse input causes a silent exit 0 rather than blocking the edit.

set -euo pipefail

# The hook is for context enrichment, not correctness. Any unexpected failure
# (jq missing, stdin parse, fs race, doc unreadable) must NOT block the edit.
# `main` does the work; any non-zero exit from inside it falls through to
# `exit 0` below, leaving Claude Code with no hook output and its normal
# permission flow intact.
main() {
    command -v jq >/dev/null 2>&1 || return 1

    local input file_path session_id sentinel docs_dir f payload
    input=$(cat)
    file_path=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')
    session_id=$(printf '%s' "$input" | jq -r '.session_id // empty')

    case "$file_path" in
        *.go) ;;
        *) return 0 ;;
    esac

    [[ -n "$session_id" ]] || return 0

    sentinel="${TMPDIR:-/tmp}/anvil-go-conventions-${session_id}.lock"
    [[ -f "$sentinel" ]] && return 0

    docs_dir="${CLAUDE_PROJECT_DIR:-$(pwd)}/docs"
    for f in go-conventions.md code-design.md agent-cli-principles.md; do
        [[ -f "$docs_dir/$f" ]] || return 0
    done

    # Mark sentinel only after we've decided to emit, so a failed run can retry.
    touch "$sentinel"

    payload=$(cat <<EOF
The following Anvil conventions apply to Go edits in this repo. They are
loaded once per session via the \`inject-go-conventions\` PreToolUse hook.
Read and honour them before completing this Edit/Write — they encode
load-bearing rules that the index-style "Read when..." pointers in
CLAUDE.md don't reliably surface.

---

$(cat "$docs_dir/go-conventions.md")

---

$(cat "$docs_dir/code-design.md")

---

$(cat "$docs_dir/agent-cli-principles.md")
EOF
)

    jq -n --arg ctx "$payload" '{
        hookSpecificOutput: {
            hookEventName: "PreToolUse",
            permissionDecision: "defer",
            additionalContext: $ctx
        }
    }'
}

main || true
exit 0
