---
name: completing-issue
description: "Use when implementing an open issue end-to-end to PR-opened. Triggers: 'complete issue X', 'work issue <id>'. Not for authoring (writing-issue) or fleet dispatch (dispatching-issue-fleet)."
license: MIT
allowed-tools: [Bash, Read, Edit, Write]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: completing-issue
  vault_type: skill
  skill_type: workflow
  side: execution
  created: 2026-05-19
  updated: 2026-06-11
  tags: [type/skill, activity/issue]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Completing Issue

> **Iron Law: NO PR OPENS WITHOUT BOTH DIRECT AND INDIRECT VERIFICATION PASSING.**
>
> Indirect verification drives the change through the built/installed/served artifact — "tests pass" is not enough. The issue's `## Verification → Indirect` block enumerates what "actually works" looks like; refusing to run it is how regressions land in merged PRs.

## When this skill runs

You enter holding:

1. An open or in-progress issue with a `## Verification` section containing both `### Direct` and `### Indirect` entries.
2. A worktree (or branch) dedicated to the issue, separated from the main checkout per the project's branching convention.

If `## Verification` is missing either subsection or its entries are non-predicate-shaped ("feature works" rather than "command X exits 0 / output contains Y"), halt and hand back to `writing-issue`. Do not improvise checks — the issue spec is the contract.

## Running delegated on a cheaper model

For a one-off completion, the main agent can dispatch this skill to an isolated subagent on a cheaper model (e.g. Opus main → Sonnet worker) — fill and fire `dispatch-single.md`. It stops at PR-opened with no review-respond loop. The N-parallel case is `dispatching-issue-fleet`.

## Phase 0 — Claim

```bash
anvil show issue <id>
anvil transition issue <id> in-progress --owner <your-name>
```

Read the issue's `goal:` — its one-sentence terminal predicate — and hold it as orientation for everything below: `## Verification` is the binary gate, `goal:` is what the change is *for*.

The `in-progress` transition re-runs `reproduction_anchor` for bug issues, and refuses the claim unless `goal:` is set (backfill-on-claim for the pre-`goal` back-catalogue). A mismatch means the bug is stale or already fixed — surface and stop; do not paper over with `--force`.

## Phase 1 — Implement

**Load the governing contract(s) first.** An issue's routing links name the contracts bounding its slice — per-component guardrails (`## Does not`, `## Code design`) the house docs don't carry:

```bash
anvil show issue <id> --json \
  | jq -r '.related[]? | select(startswith("[[contract.")) | ltrimstr("[[contract.") | rtrimstr("]]")'
anvil show contract <id> --body   # for each id printed
```

Treat each `## Does not` as a hard boundary (crossing one → **Scope-change protocol**); apply its `## Code design` as you write. No contract resolves → none governs this slice; rely on the repo's core conventions indexed from `CLAUDE.md`/`AGENTS.md`.

Make the minimal change that achieves the issue's `goal:` and passes every `## Verification` check (`## Acceptance criteria`, when present, is a prose aid — not the gate). Stay within the issue's declared file set (or `<declared-files>` when dispatched by `dispatching-issue-fleet`). See **Scope-change protocol** below if the work outgrows declared scope.

No refactoring "while in the area." No helpers without a second use. No defensive code for unreachable states. Defer to the project's conventions (`CLAUDE.md`, `AGENTS.md`, style guides) for project-specific hard rules.

## Phase 2 — Verify (max 5 cycles)

Run the bundled verification runner against the issue. It parses `## Verification → ### Direct` then `### Indirect` (fenced bash blocks), runs each block as one script (lines share state), and emits a compact `PASS [Direct#N] <preview>` / `FAIL [Direct#N] <preview>` summary with up to 10 lines of failure output per fail.

```bash
anvil show issue <id> | bash ~/.claude/skills/completing-issue/scripts/run-verification.sh
```

Exit 0 = every check passed. Non-zero = at least one failed; the summary names which.

Outcomes:

- **All pass** → Phase 3.
- **Any fail** → fix using the failure output as context, increment the cycle counter, restart Phase 2 from step 1.
- **5 cycles fail** → Phase 5 (failure report).
- **Same wall hit twice** → Phase 5 early. Agent judgment: more iterations on the same context won't unblock.
- **Signal unclean** (flaky, environment-dependent — a genuine attempt yields neither a deterministic pass nor a deterministic fail) → Phase 5 as `INCONCLUSIVE`. Do not soften an unclean or negative result into a PASS to open the PR; report the verdict honestly and leave the PR unopened.

A Direct pass with an Indirect fail is the precise gap this skill exists to catch. Treat it as a regular fail; iterate.

## Phase 3 — Self-review the diff

Re-read the change once. Two checklists:

**Project-specific** — pull violations from `CLAUDE.md`, `AGENTS.md`, contributor docs, the project's style guide, and the governing contract(s) loaded in Phase 1 (re-check the diff against each `## Does not`). Fix what you find.

**Generic anti-patterns** — these apply regardless of project:

- Dead or unused code added by the change.
- Helpers introduced for a single caller.
- Defensive code for states the type system already forbids.
- Comments explaining *what* (the code already shows that) instead of *why*.
- New top-level dependencies pulled in without explicit need.
- Edits outside the change's declared scope.

Code review agents have a finite budget — the cheaper the diff, the more of their budget catches real bugs.

## Phase 4 — Build-and-install gate

Run the project's build-and-install command — read it from the repo's conventions. Common shapes: `make install`, `just install`, `npm run build && npm link`, `cargo install --path .`, `pip install -e .`, project-specific scripts. The goal is to rebuild the artifact your change lives in so the installed/served version reflects the working tree, not stale bits.

If the project stamps the built artifact with a version or commit sha, verify the just-built artifact reports the current HEAD (`-dirty` suffix is expected when the tree has uncommitted changes). If it doesn't match, the install path bypassed your build — fix that before continuing.

Then re-run every `### Indirect` entry against the built artifact, not the dev tree. A passing dev-tree verify and a failing built-artifact verify means the install/build path is broken — fix before opening the PR.

An Indirect block whose predicates only assert presence (`--help | grep`, `test -f`, grepping source/skill files) does not satisfy "actually works" — those checks exit 0 even when the artifact is broken. Treat the build gate as failed and report it via the Phase 5 failure path; do not treat a presence-only Indirect pass as behavioral verification.

## Phase 5 — Open PR or report failure

**On verify + build-gate success:**

```bash
gh pr create --title "<conventional-commit summary>" --body "<one-paragraph + closes #<issue-number>>"
```

Surface the PR url. Stop. The issue stays `in-progress`; the human transitions it to `resolved` after merge. **REQUIRED SUB-SKILL:** Use reviewing-pr to run the default independent review pass, then responding-to-pr-review to drive its findings to resolution — unless you were dispatched to stop at PR-opened (e.g. by `dispatching-issue-fleet`), where the orchestrator owns review.

When a responding-to-pr-review loop needs to wait for CI or a reviewer pass, invoke the out-of-band poller **once** instead of polling in-agent:

```bash
bash ~/.claude/skills/completing-issue/scripts/wait-for-pr.sh --pr <n> [--repo owner/repo] [--timeout 900]
# emits one JSON result on terminal state: state, merged, ci_conclusion, review_blockers_count, timed_out
# state: merged | closed | conflicting | ci_passed | review_blocked | ci_failed | timeout — branch handling is in responding-to-pr-review Phase 4
```

In-agent polling burns tokens on every LLM iteration; a single `wait-for-pr.sh` call blocks out-of-band and returns exactly when action is needed.

**On verify failure or inconclusive signal (Phase 2 abort):**

Print a structured report to the terminal:

```text
Issue <id>: verification did not converge after <N> cycles.

Verdict: <FAILED | INCONCLUSIVE>
Root cause: <one sentence>
Failed step: <Direct: <which> | Indirect: <which>>
Last failure output: <quoted, ≤10 lines>
What is blocked: <one sentence>
Recommended next step: <one sentence>
```

Do NOT call `gh pr create`. Do NOT transition the issue. Leave the worktree for human review.

## Scope-change protocol

If the work outgrows declared scope (files > declared, LOC > issue estimate, lint cluster outside the change), halt and surface counts:

```text
Scope-change: <metric>=<observed> vs <declared> — <one-line cause>
```

Do not silently scope down (cut a quieter version) or up (touch sibling files). The human decides: split the issue, expand scope, or abort.

**Autonomous mode (unattended runs):** when running unattended, never block on a human for a scope-change. Dispatched as the `anvil-issue-worker` agent, emit it as the worker's `Blocker:` return line and halt — the orchestrator records and skips it. Running inline in an autonomous orchestrator, file a blocker instead (an `inbox` item, or a `blocks` issue when a milestone fits), leave the issue `in-progress`, note it in the report, and continue. Verification non-convergence already reports rather than hangs (Phase 5). Every other gate — `## Verification`, the build-and-install gate, the independent review — still runs; never auto-merge.

## Forbidden calls

- `gh pr merge` — human owns the merge button.
- `git worktree remove` — post-merge cleanup is the human's.
- `anvil transition resolved` — human transitions after merge.
- `anvil transition abandoned` — emit a failure report instead.

## Forbidden patterns

- Resolving an issue with a green Direct pass but no Indirect run.
- Improvising verification commands the issue does not declare.
- Looping past 5 verify cycles "just one more try."
- Editing files outside the issue's declared scope to make verification pass.
