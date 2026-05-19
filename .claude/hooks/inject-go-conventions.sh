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

    # Scope filter: only Go files under internal/ or cmd/ touch the idioms the
    # injected docs cover (types, error handling, subprocess gotchas, CLI shape).
    # Edits to skills/skills.go (embed list), generated fixtures, etc. don't
    # benefit from the ~10KB payload. Match on the path as given — relative
    # paths are matched at any depth, absolute paths must contain the segment.
    local rel="$file_path"
    case "$rel" in
        /*) rel="${rel#"${CLAUDE_PROJECT_DIR:-$(pwd)}/"}" ;;
    esac
    case "$rel" in
        internal/*|cmd/*|*/internal/*|*/cmd/*) ;;
        *) return 0 ;;
    esac

    # Line-shape filter: if every changed non-blank line is a Go comment
    # (including //go:embed directives), the conventions payload is dead
    # weight. Inspects old_string/new_string (Edit) and content (Write).
    # Multi-line raw strings whose lines start with `//` are a known
    # false-positive — acceptable cost vs a real Go parser.
    local old_str new_str content combined
    old_str=$(printf '%s' "$input" | jq -r '.tool_input.old_string // empty')
    new_str=$(printf '%s' "$input" | jq -r '.tool_input.new_string // empty')
    content=$(printf '%s' "$input" | jq -r '.tool_input.content // empty')
    combined="${old_str}"$'\n'"${new_str}"$'\n'"${content}"
    if [[ -n "${old_str}${new_str}${content}" ]] \
        && ! printf '%s\n' "$combined" \
            | grep -vE '^[[:space:]]*(//.*)?$' \
            | grep -q .; then
        return 0
    fi

    # Once-per-session sentinel: dedupe only when we have a session_id; without
    # one (e.g. synthetic test invocations) every call emits, since there's no
    # session to scope the lock to.
    if [[ -n "$session_id" ]]; then
        sentinel="${TMPDIR:-/tmp}/anvil-go-conventions-${session_id}.lock"
        [[ -f "$sentinel" ]] && return 0
    fi

    docs_dir="${CLAUDE_PROJECT_DIR:-$(pwd)}/docs"
    for f in go-conventions.md code-design.md agent-cli-principles.md; do
        [[ -f "$docs_dir/$f" ]] || return 0
    done

    # Mark sentinel only after we've decided to emit, so a failed run can retry.
    [[ -n "$session_id" ]] && touch "$sentinel"

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
