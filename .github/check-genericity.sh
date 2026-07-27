#!/usr/bin/env bash
# Guards the //go:embed bundles (anvil/skills/**, anvil/agents/**) against
# anvil-repo-only leakage. Those trees are what `anvil install skills|agents`
# writes verbatim into any consuming project, so their bodies must never
# assume anvil's own toolchain or cite anvil's own source-tree layout — a
# burgh/mentat worker has neither. Regressed once (PR #338, 2026-07-26) past
# seven independent `anvil-pr-reviewer` dispatches, because "is this generic"
# is exactly the axis a reviewer loaded on anvil's own CLAUDE.md is blind to.
# A deterministic grep catches it where a review axis depends on remembering
# to ask. Shared by the CI step and the pre-commit hook.
set -euo pipefail

fail=0

# Pre-existing citations not yet repaired (tracked by anvil.0215's follow-up
# slices). New hits in any other file fail the gate; drop an entry here the
# same PR that repairs it so the grandfather list only shrinks.
allowlist=(
  "anvil/skills/completing-issue/dispatch-single.md"
  "anvil/skills/self-testing/SKILL.md"
  "anvil/skills/reviewing-pr/SKILL.md"
  "anvil/skills/dispatching-issue-fleet/SKILL.md"
)

is_allowlisted() {
  local f="$1"
  for a in "${allowlist[@]}"; do
    [ "$f" = "$a" ] && return 0
  done
  return 1
}

# Anvil's own dev-loop commands named as *the* gate, rather than as one
# example of a class (contrast: completing-issue's Phase 4 names
# `make install`, `just install`, `npm run build && npm link`,
# `cargo install --path .`, `pip install -e .` — a class with examples, not a
# single toolchain — and is deliberately not flagged by this pattern).
toolchain='just (check|install-local|build)|go test|golangci'
while IFS= read -r f; do
  is_allowlisted "$f" && continue
  echo "::error file=$f::names anvil's own toolchain command directly — a shipped skill/agent body must name a class of command (see completing-issue's build-and-install gate) or discover the project's gate from its CLAUDE.md/AGENTS.md, never assume 'just'/'go test'/golangci are on the consuming project's PATH"
  fail=1
done < <(grep -rlE "$toolchain" --include='*.md' anvil/skills/ anvil/agents/)

# anvil's own source-tree paths (anvil/skills/<slug>, anvil/agents/<name>.md)
# cited as if they resolved in the installed project — they exist only in
# this repo's checkout, not in ~/.claude/skills or a consuming project.
while IFS= read -r f; do
  is_allowlisted "$f" && continue
  echo "::error file=$f::cites an anvil source-tree path (anvil/skills/... or anvil/agents/...) — name the sibling skill/agent by its bundled name instead, the path does not exist outside this repo's checkout"
  fail=1
done < <(grep -rlE 'anvil/(skills|agents)/[a-z-]+' --include='*.md' anvil/skills/ anvil/agents/)

exit "$fail"
