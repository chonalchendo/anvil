#!/usr/bin/env bash
# run-verification.sh — execute the ## Verification → ### Direct / ### Indirect
# fenced-bash blocks from an anvil issue (markdown on stdin).
#
# Output contract — the verdict is data, not prose, because a worker's narrated
# verdict does not survive the hop to its orchestrator:
#   stdout: exactly one line of JSON, the verdict —
#           {"verdict":"pass|fail","checks":N,"failed":[{"check":"Indirect#1",
#            "exit":4,"preview":"<first command>"}],"commit":"<sha-or-empty>",
#            "ran_at":"<UTC RFC3339>"}
#           "checks" counts blocks attempted; a section with no ```bash block
#           counts as one attempted (and failed) check. "commit" is `git
#           rev-parse HEAD` of the cwd the runner was invoked from; a check
#           that cd's elsewhere or exercises an installed artifact is not
#           covered. A dirty cwd appends "-dirty" to the sha (mirrors
#           justfile's `install` recipe). "" when that cwd isn't a git repo.
#           "ran_at" is when the run started (UTC RFC3339). The runner
#           records provenance; it does not enforce freshness — that's the
#           consumer's call.
#   stderr: the human PASS/FAIL summary and up to 10 lines per failure.
#   exit:   0 iff verdict is "pass", 1 otherwise.
#
# Each ```bash block runs as ONE script under `set -e`: its lines share state,
# so the natural idiom — capture output once, then assert on it across several
# lines — works, while ANY failing line fails the block (not just the last).
# Blocks run in the cwd the runner is invoked from, so invoke it from the
# worktree under test.
#
# One shape escapes `set -e`: bash exempts a `!`-negated command in ANY command
# position (`! c`, `x; ! c`, `a && ! c`, `do ! c`, `then ! c`, `{ ! c; }`), so a
# failing `! cmd` does not abort the block and only gates as the block's last
# line (its exit status). Any earlier such line is refused before the block runs
# — a predicate survives only as one self-contained exit-code line, so write the
# non-last-line form as `if cmd; then exit 1; fi`.
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

# Print (trimmed) the first line of a block carrying a `!` in command position
# that is not the block's last executable line, or nothing. Such a line cannot
# fail the block: `set -e` exempts a non-final `!` command, and only the last
# line's status becomes the verdict. The command positions are those bash
# verified as exempt — line start, `;`, `&&`, `||`, `do`, `then`, `else`, `{` —
# but NOT `(`, since a subshell `( ! cmd )` does abort. Textual, so a quoted or
# heredoc `; ! ` trips it too: refusing loudly beats a silently-skipped
# assertion. internal/core's NonGatingNegation is the Go twin of this rule, and
# internal/installer's lockstep test drives one corpus through both.
non_gating_negation() {
    awk '
        { line = $0; gsub(/^[[:space:]]+|[[:space:]]+$/, "", line) }
        line == "" || line ~ /^#/ { next }
        { n++; l[n] = line }
        END {
            for (i = 1; i < n; i++)
                if (l[i] ~ /(^|[;&|{]|(^|[[:space:]])(do|then|else))[[:space:]]*!([[:space:]]|$)/) { print l[i]; exit }
        }
    '
}

checks=0
failed_json=""

# Captured before any check runs, so a check that cd's cannot shift it.
commit=$(git rev-parse HEAD 2>/dev/null)
if [ -n "$commit" ] && [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    commit="${commit}-dirty"
fi
ran_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)

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
    local n=0 fails=0 rc output preview vacuous
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
        vacuous=$(printf '%s\n' "$block" | non_gating_negation)
        if [ -n "$vacuous" ]; then
            echo "FAIL [$label#$n] $preview" >&2
            echo "    non-gating negation: \`$vacuous\` carries a \`!\` in command position on a line that is not the block's last." >&2
            echo "    set -e exempts a non-final \`!\` command, so its failure cannot fail the block; and where a loop or if tail does gate, only its final iteration's status survives." >&2
            echo "    rewrite as: if <cmd>; then exit 1; fi   (gates on any line) — or make it the block's last line" >&2
            add_fail "$label#$n" null "non-gating negation: $vacuous"
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
        checks=$((checks + 1))
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
    printf '{"verdict":"pass","checks":%d,"failed":[],"commit":"%s","ran_at":"%s"}\n' "$checks" "$commit" "$ran_at"
    exit 0
else
    echo "$total check(s) failed." >&2
    printf '{"verdict":"fail","checks":%d,"failed":[%s],"commit":"%s","ran_at":"%s"}\n' "$checks" "$failed_json" "$commit" "$ran_at"
    exit 1
fi
