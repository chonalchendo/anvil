#!/usr/bin/env bash
# Guards the //go:embed bundles (anvil/skills/**, anvil/agents/**) against
# anvil-repo-only leakage. Those trees are what `anvil install skills|agents`
# writes verbatim into any consuming project, so their bodies must never
# assume anvil's own toolchain or cite anvil's own source-tree layout — a
# burgh/mentat worker has neither. Regressed once (PR #338, 2026-07-26) past
# seven independent `anvil-pr-reviewer` dispatches, because "is this generic"
# is exactly the axis a reviewer loaded on anvil's own CLAUDE.md is blind to.
# A deterministic grep catches it where a review axis depends on remembering
# to ask. Shared by the CI step, the pre-commit hook, and `just check`.
set -euo pipefail

# The scan roots below are repo-relative, so any cwd other than the repo root
# — a hook fired from a subdirectory, `cd .github && ./check-genericity.sh` —
# would scan nothing and still exit 0. `set -e` cannot catch it: each scan
# reads from a process substitution, whose exit status bash discards.
cd "$(git rev-parse --show-toplevel)"

# Belt to that braces: a gate reporting green having read no files is worse
# than no gate, so refuse outright rather than pass vacuously.
[ -d anvil/skills ] && [ -d anvil/agents ] || {
  echo "::error::scan roots missing (expected anvil/skills and anvil/agents under $PWD) — refusing to report green on an empty scan" >&2
  exit 2
}

roots=(anvil/skills/ anvil/agents/)

# Scan every shipped file, not just markdown: skills.go's //go:embed pulls
# whole directories, so completing-issue/scripts/*.sh reach a consuming
# project too — and a shell script is where a toolchain command is most
# likely to appear. Two exclusions, neither of them a grandfather:
#   *.go    — the embedder sources (skills.go, agents.go), which the //go:embed
#             directives do not list and `anvil install` therefore never writes.
#   evals/  — eval fixtures embed simulated `pytest`/`uv pip` transcripts as
#             test data, not as contract prose an agent would act on.
scope=(--exclude='*.go' --exclude-dir=evals)

fail=0

# Anvil's own dev-loop commands named as *the* gate, rather than as one
# example of a class (contrast: completing-issue's Phase 4 names
# `make install`, `just install`, `npm run build && npm link`,
# `cargo install --path .`, `pip install -e .` — a class with examples, not a
# single toolchain — and is deliberately not flagged by this pattern).
toolchain='just (check|install-local|build)|go test|golangci'
while IFS= read -r f; do
  echo "::error file=$f::names anvil's own toolchain command directly — a shipped skill/agent body must name a class of command (see completing-issue's build-and-install gate) or discover the project's gate from its CLAUDE.md/AGENTS.md, never assume 'just'/'go test'/golangci are on the consuming project's PATH"
  fail=1
done < <(grep -rlE "$toolchain" "${scope[@]}" "${roots[@]}")

# `just install` is the one invocation the pattern above cannot own outright:
# it is an anvil-only imperative everywhere except inside completing-issue
# Phase 4's cross-ecosystem enumeration, where it sits beside `make install`,
# `npm run build && npm link`, `cargo install --path .` and `pip install -e .`
# as one example of a class. Resolve that per line rather than leaving the gap
# open — a line naming two or more *other* ecosystems' installers is the
# enumeration; `just install` anywhere else is an instruction the consuming
# project cannot follow. (PR #338's leak included exactly this form.)
enumeration='(make install|npm run|cargo install|pip install|go install).*(make install|npm run|cargo install|pip install|go install)'
while IFS= read -r f; do
  echo "::error file=$f::instructs 'just install' outside a cross-ecosystem enumeration — say 'the project's build-and-install command' instead, or list it beside the other ecosystems' equivalents as completing-issue's Phase 4 does"
  fail=1
done < <(grep -rnE 'just install' "${scope[@]}" "${roots[@]}" | grep -vE "$enumeration" | cut -d: -f1 | sort -u)

# anvil's own source-tree paths (anvil/skills/<slug>, anvil/agents/<name>.md)
# cited as if they resolved in the installed project — they exist only in
# this repo's checkout, not in ~/.claude/skills or a consuming project.
while IFS= read -r f; do
  echo "::error file=$f::cites an anvil source-tree path (anvil/skills/... or anvil/agents/...) — name the sibling skill/agent by its bundled name instead, the path does not exist outside this repo's checkout"
  fail=1
done < <(grep -rlE 'anvil/(skills|agents)/[a-z-]+' "${scope[@]}" "${roots[@]}")

exit "$fail"
