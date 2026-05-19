#!/usr/bin/env bash
# run-verification.sh — execute the ## Verification → ### Direct / ### Indirect
# fenced-bash blocks from an anvil issue (markdown on stdin) and emit a compact
# PASS/FAIL summary. Exits 0 if every check passes, 1 otherwise.
#
# Usage:
#   anvil show issue <id> | bash run-verification.sh
#
# Format reference: docs/issue-spec.md

set -uo pipefail

input=$(cat)

extract_block() {
    # Args: subsection label (e.g. "Direct", "Indirect").
    # Prints each non-empty, non-comment shell line found inside fenced bash
    # blocks under the matching ### header, until the next ### or ## header.
    local label=$1
    echo "$input" | awk -v label="$label" '
        $0 ~ "^### " label { in_section = 1; next }
        /^### / && in_section { in_section = 0; in_block = 0 }
        /^## /  && in_section { in_section = 0; in_block = 0 }
        in_section && /^```bash[[:space:]]*$/ { in_block = 1; next }
        in_section && /^```/ && in_block { in_block = 0; next }
        in_section && in_block && !/^[[:space:]]*$/ && !/^[[:space:]]*#/ { print }
    '
}

run_section() {
    local label=$1
    local cmds
    cmds=$(extract_block "$label")

    if [ -z "$cmds" ]; then
        echo "FAIL ### $label has no executable shell commands"
        return 1
    fi

    local n=0 fails=0
    while IFS= read -r cmd; do
        n=$((n + 1))
        if output=$(bash -c "$cmd" 2>&1); then
            echo "PASS [$label#$n] $cmd"
        else
            rc=$?
            echo "FAIL [$label#$n] $cmd (exit $rc)"
            echo "$output" | head -10 | sed 's/^/    /'
            fails=$((fails + 1))
        fi
    done <<< "$cmds"
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
