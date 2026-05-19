---
name: completing-issue
description: "Use when implementing an open issue end-to-end to PR-opened via a direct+indirect verify-or-iterate loop. Triggers: 'complete issue X', 'work issue <id>'. Not for authoring (writing-issue) or fleet dispatch (dispatching-issue-fleet)."
license: MIT
allowed-tools: [Bash, Read, Edit, Write]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: completing-issue
  vault_type: skill
  skill_type: workflow
  side: execution
  created: 2026-05-19
  updated: 2026-05-19
  tags: [type/skill, activity/issue]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Completing Issue

> **Iron Law: NO PR OPENS WITHOUT BOTH DIRECT AND INDIRECT VERIFICATION PASSING.**
>
> Indirect verification drives the change through the installed binary or a live sub-skill — "tests pass" is not enough. The issue's `## Verification → Indirect` block enumerates what "actually works" looks like; refusing to run it is how regressions land in merged PRs.

## When this skill runs

You enter holding:

1. An open or in-progress issue with a `## Verification` section containing both `### Direct` and `### Indirect` entries.
2. A worktree on the issue's branch (`anvil/<slug>`), per `@docs/worktree-workflow.md`.

If `## Verification` is missing either subsection, halt and hand back to `anvil:writing-issue`. Do not improvise checks — the issue spec is the contract.

## Phase 0 — Claim

```bash
anvil show issue <id>
anvil transition issue <id> in-progress --owner <your-name>
```

The `in-progress` transition re-runs `reproduction_anchor` for bug issues. A mismatch means the bug is stale or already fixed — surface and stop; do not paper over with `--force`.

## Phase 1 — Implement

Make the minimal change satisfying every `## Acceptance criteria` entry. Stay within the issue's declared file set (or `<declared-files>` when dispatched by `anvil:dispatching-issue-fleet`). See **Scope-change protocol** below if the work outgrows declared scope.

No refactoring "while in the area." No helpers without a second use. No defensive code for unreachable states. Per `CLAUDE.md`.

## Phase 2 — Verify (max 5 cycles)

Run, in order:

1. Every `## Verification → Direct` entry (unit/integration tests).
2. Every `## Verification → Indirect` entry (live smoke against CLI or sub-skill).

Outcomes:

- **All pass** → Phase 3.
- **Any fail** → fix using the failure output as context, increment the cycle counter, restart Phase 2 from step 1.
- **5 cycles fail** → Phase 5 (failure report).
- **Same wall hit twice** → Phase 5 early. Agent judgment: more iterations on the same context won't unblock.

A Direct pass with an Indirect fail is the precise gap this skill exists to catch. Treat it as a regular fail; iterate.

## Phase 3 — Self-review the diff

Re-read the change once for `CLAUDE.md` hard-rule violations:

- Helper without a second use.
- Defensive code for unreachable states.
- Comments explaining *what* instead of *why*.
- `fmt.Println` for CLI output (use `cmd.Println` / `cmd.PrintErrln` / `log/slog`).
- New top-level dependency (requires explicit user approval).
- Whole-file `Read` of files >150 lines without grepping first.

Fix what you find. CodeRabbit budget is finite — the cheaper the diff, the more of its budget catches real bugs.

## Phase 4 — Smoke-test gate

```bash
just install
anvil --version
```

`anvil --version` must end in `$(git rev-parse --short HEAD)` (per `@docs/worktree-workflow.md`). Then re-run every `## Verification → Indirect` entry against the installed binary. A passing dev-tree verify and a failing installed-binary verify means the install path is broken — fix before opening the PR.

## Phase 5 — Open PR or report failure

**On verify + smoke success:**

```bash
gh pr create --title "<conventional-commit summary>" --body "<one-paragraph + closes #<issue-number>>"
```

Surface the PR url. Stop. The issue stays `in-progress`; the human transitions it to `resolved` after merge. **REQUIRED SUB-SKILL:** Use anvil:responding-to-pr-review once CodeRabbit reports.

**On verify failure (Phase 2 abort):**

Print a structured report to the terminal:

```text
Issue <id>: verification did not converge after <N> cycles.

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
