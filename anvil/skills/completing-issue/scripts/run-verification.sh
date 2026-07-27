#!/usr/bin/env bash
# run-verification.sh — execute the ## Verification → ### Direct / ### Indirect
# fenced-bash blocks from an anvil issue (markdown on stdin).
#
# Output contract — the verdict is data, not prose, because a worker's narrated
# verdict does not survive the hop to its orchestrator:
#   stdout: exactly one line of JSON, the verdict —
#           {"verdict":"pass|fail","checks":N,"failed":[{"check":"Indirect#1",
#            "exit":4,"preview":"<first command>"}]}
#   stderr: the human PASS/FAIL summary and up to 10 lines per failure.
#   exit:   0 iff verdict is "pass", 1 otherwise.
#
# Each ```bash block runs as ONE script under `set -e`: its lines share state,
# so the natural idiom — capture output once, then assert on it across several
# lines — works, while ANY failing line fails the block (not just the last).
# Blocks run in the cwd the runner is invoked from, so invoke it from the
# worktree under test.
#
# Usage:
#   anvil show issue <id> | bash run-verification.sh            # summary on stderr
#   anvil show issue <id> | bash run-verification.sh | jq -r .verdict
#
# Format: see writing-issue SKILL.md → ## Verification body section.

set -uo pipefail

input=$(cat)

# Emit each top-level ```bash block under the given ### subsection as a NUL-
# delimited script. Fence depth is tracked so a block may itself contain nested
# ``` fences and ##/### headers (e.g. a heredoc carrying a mini issue doc)
# without the inner markers being mistaken for structure: an info-string fence
# (```lang) opens a level, a bare ``` closes one, and only the outermost
# ```bash opener starts a captured check.
extract_blocks() {
    local label=$1
    printf '%s\n' "$input" | awk -v label="$label" '
        /^```/ {
            if ($0 ~ /^```[A-Za-z]/) {
                if (insec && depth == 0 && $0 ~ /^```bash[[:space:]]*$/) {
                    inblock = 1; depth = 1; buf = ""; next
                }
                depth++
                if (inblock) buf = buf $0 "\n"
                next
            }
            if (inblock && depth == 1) {
                printf "%s%c", buf, 0
                inblock = 0; depth = 0; next
            }
            if (depth > 0) depth--
            if (inblock) buf = buf $0 "\n"
            next
        }
        depth == 0 && $0 ~ ("^### " label "([^A-Za-z]|$)") { insec = 1; next }
        depth == 0 && insec && /^### / { insec = 0; next }
        depth == 0 && insec && /^## /  { insec = 0; next }
        inblock { buf = buf $0 "\n" }
    '
}

checks=0
failed_json=""

# Accumulate one failed-check object. Built by hand rather than via jq so the
# runner keeps its zero-dependency contract; arrays are avoided because macOS
# still ships bash 3.2, where `${arr[@]}` on an empty array trips `set -u`.
add_fail() { # check exit-code-or-null preview
    local esc
    esc=$(printf '%s' "$3" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr -d '\000-\037')
    [ -n "$failed_json" ] && failed_json="$failed_json,"
    failed_json="$failed_json{\"check\":\"$1\",\"exit\":$2,\"preview\":\"$esc\"}"
}

run_section() {
    local label=$1
    local n=0 fails=0 rc output preview
    while IFS= read -r -d '' block; do
        n=$((n + 1))
        checks=$((checks + 1))
        preview=$(printf '%s\n' "$block" | grep -vE '^[[:space:]]*(#|$)' | head -1)
        if [ -z "$preview" ]; then
            echo "FAIL [$label#$n] block has no executable command (empty or all comments)" >&2
            add_fail "$label#$n" null "block has no executable command"
            fails=$((fails + 1))
            continue
        fi
        # -e so ANY failing line fails the block, not just the last: a block
        # ending in cleanup (`… || true`) must not mask an earlier assertion's
        # failure. Redirect stdin from /dev/null so a command that reads stdin
        # (e.g. an anvil verb probing for piped body input) doesn't consume the
        # process-substitution stream feeding this while-read loop.
        if output=$(bash -ec "$block" </dev/null 2>&1); then
            echo "PASS [$label#$n] $preview" >&2
        else
            rc=$?
            echo "FAIL [$label#$n] $preview (exit $rc)" >&2
            printf '%s\n' "$output" | head -10 | sed 's/^/    /' >&2
            add_fail "$label#$n" "$rc" "$preview"
            fails=$((fails + 1))
        fi
    done < <(extract_blocks "$label")

    if [ "$n" -eq 0 ]; then
        echo "FAIL ### $label has no executable \`\`\`bash block" >&2
        add_fail "$label" null "no executable bash block"
        return 1
    fi
    return $fails
}

direct_fails=0
indirect_fails=0

echo "=== Direct (unit/integration) ===" >&2
run_section "Direct" || direct_fails=$?
echo "" >&2
echo "=== Indirect (live smoke) ===" >&2
run_section "Indirect" || indirect_fails=$?

total=$((direct_fails + indirect_fails))
echo "" >&2
if [ "$total" -eq 0 ]; then
    echo "All checks passed." >&2
    printf '{"verdict":"pass","checks":%d,"failed":[]}\n' "$checks"
    exit 0
else
    echo "$total check(s) failed." >&2
    printf '{"verdict":"fail","checks":%d,"failed":[%s]}\n' "$checks" "$failed_json"
    exit 1
fi
