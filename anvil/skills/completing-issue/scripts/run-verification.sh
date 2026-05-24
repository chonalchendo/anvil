#!/usr/bin/env bash
# run-verification.sh — execute the ## Verification → ### Direct / ### Indirect
# fenced-bash blocks from an anvil issue (markdown on stdin) and emit a compact
# PASS/FAIL summary. Exits 0 if every check passes, 1 otherwise.
#
# Each ```bash block runs as ONE script: its lines share state, so the natural
# idiom — capture output once, then assert on it across several lines — works.
# The unit of PASS/FAIL is the block, not the line. Blocks run in the cwd the
# runner is invoked from, so invoke it from the worktree under test.
#
# Usage:
#   anvil show issue <id> | bash run-verification.sh
#
# Format reference: docs/issue-spec.md

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

run_section() {
    local label=$1
    local n=0 fails=0 rc output preview
    while IFS= read -r -d '' block; do
        n=$((n + 1))
        preview=$(printf '%s\n' "$block" | grep -vE '^[[:space:]]*(#|$)' | head -1)
        # Redirect stdin from /dev/null so a command that reads stdin (e.g. an
        # anvil verb probing for piped body input) doesn't consume the
        # process-substitution stream feeding this while-read loop.
        if output=$(bash -c "$block" </dev/null 2>&1); then
            echo "PASS [$label#$n] $preview"
        else
            rc=$?
            echo "FAIL [$label#$n] $preview (exit $rc)"
            printf '%s\n' "$output" | head -10 | sed 's/^/    /'
            fails=$((fails + 1))
        fi
    done < <(extract_blocks "$label")

    if [ "$n" -eq 0 ]; then
        echo "FAIL ### $label has no executable \`\`\`bash block"
        return 1
    fi
    return $fails
}

direct_fails=0
indirect_fails=0

echo "=== Direct (unit/integration) ==="
run_section "Direct" || direct_fails=$?
echo ""
echo "=== Indirect (live smoke) ==="
run_section "Indirect" || indirect_fails=$?

total=$((direct_fails + indirect_fails))
echo ""
if [ "$total" -eq 0 ]; then
    echo "All checks passed."
    exit 0
else
    echo "$total check(s) failed."
    exit 1
fi
