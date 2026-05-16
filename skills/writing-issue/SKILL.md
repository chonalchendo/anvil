---
name: writing-issue
description: "Use when a problem worth tracking surfaces. Triggers: 'open an issue for X', 'track this as an issue', 'should we build X', 'promote inbox item to issue'. Not for raw capture (capturing-inbox) or plans (writing-plan)."
license: MIT
allowed-tools: [Bash, Read, Edit]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-issue
  vault_type: skill
  skill_type: workflow
  side: execution
  created: 2026-04-30
  updated: 2026-04-30
  tags: [type/skill, activity/issue]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Writing Issue

Workflow for taking a problem worth tracking — whether a fuzzy "should we build X" thought or a fully-shaped request — through to a schema-valid issue artifact under `~/anvil-vault/70-issues/`. Issues sit one level below milestones: product-design → milestones → **issues** → plans.

## Shape test

**If you can name an acceptance criterion in one breath, it's an issue.** Use this skill when the entry is decisive (problem + AC + milestone hint) OR when a fuzzy thought is ready to be pressure-tested into one. Inbox-first is NOT required when the entry is already shaped — route here directly.

Wrong-choice example: user is dumping a half-formed thought with no AC and no clear acceptance shape. That's an inbox item — hand off to `anvil:capturing-inbox` and resume here later via `anvil promote` if the thought sharpens.

## Iron Law

**NO ISSUE FILE LANDS WITHOUT AN EXPLICIT MILESTONE LINK.**

If no milestone fits, the workflow stops at Phase 2 and offers two exits: log a `decision` artifact with `status: rejected`, or hand off to `anvil:writing-milestone` and resume here once the milestone exists. There is no `--no-milestone` escape hatch.

## When this skill runs

- A problem worth tracking has surfaced — inbox item, ad-hoc message, or direct request.
- The user wants to think about whether to build something, OR already knows.
- A milestone exists for the target project, OR you are willing to create one mid-flight.

## When not to use

- The user is dumping a thought without engagement → `anvil:capturing-inbox`.
- You need to design the solution after the issue is approved → `anvil:writing-plan`.
- Editing existing issue frontmatter only (status flip, tag) → a direct `anvil set` call.

---

## Severity rubric

Anchor severity on **blast-radius × workaround-cost**:

- `critical` — corrupts data, breaks the schema, or makes `anvil` itself unusable. No workaround.
- `high` — blocks a documented workflow; agent or human must context-switch around it. Workaround exists but is costly enough that fixing-now is cheaper than working-around-twice.
- `medium` — adds friction (time, tokens, round-trips) to a workflow but does not block it. Clear, cheap workaround.
- `low` — polish, cosmetic, missing affordance that costs little to live with.

---

## Phase 0 — Entry detection

Classify the user's first message before doing anything else. The classification chooses which phases run.

- **Decisive** when the message contains *all three* of:
  - a problem statement (one sentence describing what is broken or missing),
  - at least one acceptance criterion (testable condition for done),
  - a milestone reference — either an explicit id, or a phrase the agent can map to exactly one existing milestone in `~/anvil-vault/85-milestones/` (filtered by `project` frontmatter) without further user input. If two or more milestones could plausibly match, treat as fuzzy.
- **Fuzzy** otherwise — including "should we build X", "I've been thinking about Y", "is this worth doing", or any message missing one of the three signals.
- **Tie-break:** when in doubt, run convergence. Misclassifying decisive→fuzzy costs one extra round-trip; misclassifying fuzzy→decisive ships a thin issue.

Decisive path: skip to Phase 2.
Fuzzy path: continue to Phase 1.

Phase 0 produces no artifact and no chat output beyond your internal routing decision.

---

## Phase 1 — Convergence (fuzzy path only)

Goal: a one-sentence shared understanding of what is being proposed. Without it, the pressure-test in Phase 3 stress-tests your interpretation, not the user's idea.

- Restate the idea in one sentence. Ask: "Did I get this right?"
- One clarifying question at a time. Multiple-choice preferred.
- Stop when the user explicitly confirms. "Sure, whatever" is not confirmation; ask again.
- Output: a `Problem` sentence and a `Proposal` sentence held in chat for use in Phase 4.

---

## Phase 2 — Milestone-fit gate (always; Iron Law)

Compare the converged proposal (fuzzy path) or the decisive-path's stated proposal against `~/anvil-vault/85-milestones/`, filtered by the project's `project` frontmatter.

- **Match found** → record the milestone id; continue (Phase 3 if fuzzy, Phase 4 if decisive).
- **No match, idea is small or orthogonal** → offer the user two exits:
  - (a) log a `decision` artifact with `status: rejected` (one paragraph: what was considered, why rejected). See "Terminal states" below for the CLI sequence.
  - (b) stop without an artifact (inbox source, if any, stays as-is for later resumption).
- **No match, idea reshapes the system** → stop and offer to hand off to `anvil:writing-milestone`. Resume `writing-issue` after the milestone exists. **REQUIRED SUB-SKILL:** Use anvil:writing-milestone.

Never skip the gate to issue creation.

---

## Phase 3 — Pressure-test (fuzzy path only)

Three short frames against the converged proposal. Each is a paragraph or less. Outputs are *gate-side* — discarded after the gate passes — except `smallest-viable`, which persists into the issue body as `## Non-goals`.

1. **Pre-mortem.** "Six months from now, this shipped but it was the wrong call. Why?" 2–3 plausible failure reasons. If a frame surfaces a load-bearing concern, that concern becomes a non-goal in Phase 4 or kills the issue entirely.
2. **Smallest viable version.** What is the thinnest cut that still delivers the win named in the product design? What is explicitly out of scope? *This output persists* into Phase 4 as `## Non-goals`.
3. **Working-backwards headline.** One-line release note: "We shipped X so users can Y." If boring, vague, or disconnected from a product-design success metric, return to Phase 1.

Skip a frame only when it is genuinely not applicable; record why in chat.

If a frame surfaces an unknown that needs evidence (a dependency, a competitor behaviour, a technical constraint), recommend research as a separate side task. Do not block the issue on research the user did not ask for.

---

## Phase 4 — Author the issue (always)

Before calling the CLI, **propose severity using the rubric** above, then confirm with the user. The agent does the first-pass classification rather than defaulting to `medium`; severity is required by the schema and gates triage queries.

List the existing `domain/` taxonomy so you reuse a value the user has already introduced rather than coining a near-duplicate:

```bash
anvil tags list --source used --prefix domain/ --json
```

Pick the closest existing value if one fits; only invent a new one if no existing value matches. The CLI will reject an unrecognised value unless you pass `--allow-new-facet=domain` — verbosity is intentional friction.

When promoting an inbox item, pass `--tags` on the `anvil promote <id> --as issue` call after consulting the same list.

Author the body up front and pass it to `create` via `--body-file` (or `--body -` for piped stdin). `create` validates the frontmatter AND body (required H2s, wikilink targets) and rolls back the write on failure — no separate `anvil validate` step:

```bash
cat > /tmp/issue-body.md <<'EOF'
## Problem
<one paragraph from convergence (fuzzy) or the stated problem (decisive)>

## Acceptance criteria
- <testable criterion 1>

## Non-goals
- <from Phase 3 smallest-viable or stated up front>

## Links
- [[milestone.<project>.<slug>]]
EOF

anvil create issue --title "<title>" --tags domain/<x> --body-file /tmp/issue-body.md --json
```

Capture `id` and `path` from the JSON output. The file lands at `~/anvil-vault/70-issues/<project>.<slug>.md`.

Set typed frontmatter slots (these are still post-create — typed setters live outside the body):

```bash
anvil set issue <id> milestone "[[milestone.<project>.<slug>]]"
anvil set issue <id> severity <low|medium|high|critical>
```

For `acceptance[]`, run one `--add` per criterion:

```bash
anvil set issue <id> acceptance --add "<criterion>"
```

Positional values on array fields error with `field_is_array`; use `--add VALUE` to append and `--remove INDEX` to delete.

Required body sections (enforced by `create`):

- `## Problem` — one paragraph from convergence (fuzzy) or the stated problem (decisive).
- `## Acceptance criteria` — bulleted, each testable without ambiguity.
- `## Non-goals` — from Phase 3 smallest-viable (fuzzy) or stated up front (decisive).
- `## Links` — to milestone, design docs, related issues. Use `[[wikilink]]` form. Targets must resolve (the file must exist) or `create` rejects.

`anvil validate <path>` remains useful as a re-check after edits (e.g. after `anvil set ... acceptance --add`), but it is **not** required after `create` when the body was supplied via `--body-file` / `--body -`.

**Reproduction anchor — bug issues only.** Author `reproduction_anchor` only when the issue is a **bug**: something concrete is broken today and a shell command reproduces the failure. **Skip the anchor** for feature, refactor, doc, or design-shaping issues — there is no failure mode to reproduce, and forcing one is a category error.

Shape (bug case): `command` (shell-runnable invocation that reproduces the gap), `expected` (literal output or `sha:<hex>` digest). When an agent later runs `anvil transition issue <id> in-progress`, anvil re-runs the command and refuses the claim if the output no longer matches. Two escape hatches if the gate misfires:

- `--force` — bypass the check and claim anyway (use when the anchor itself is broken but the issue is real).
- `--no-longer-reproduces` — confirm the mismatch and close the issue directly as `resolved` with the diff captured in the audit trail.

Anchor authoring stays optional. Bug issues without an anchor, and all non-bug issues, transition normally (grandfather rule).

---

## Working the issue (state machine)

The issue lifecycle is `open → in-progress → resolved` (with `→ abandoned` and reverse audit edges). All status changes go through `anvil transition`, not direct frontmatter edits.

```bash
# Claim — --owner is required (open → in-progress)
anvil transition issue <id> in-progress --owner <name>

# Resolve when the work is merged (in-progress → resolved)
anvil transition issue <id> resolved

# Reopen with audit trail (resolved → open requires --reason)
anvil transition issue <id> open --reason "<why>"
```

Use `anvil set ... status` only as a force-edit escape hatch when `transition` rejects a legal-but-unusual move.

## Terminal states

Three exits:

1. **`issue` created** — file exists, validates, milestone link set. **Scope-survey before handing off:** multi-file, multi-task, or non-obvious decomposition → hand off to `anvil:writing-plan`. Single-file or two-file change with a clear test contract → skip the plan, implement inline (under `anvil:implementing-plan` if a plan exists, otherwise direct TDD). The plan layer earns its keep on decomposition, not on file count alone.
2. **`decision/rejected`** — user bailed mid-session. Prompt: "log this as a rejected decision?" If yes:
   ```bash
   anvil create decision --title "Considered: <X>" --json
   anvil set decision <id> status rejected
   anvil set decision <id> date <today>
   ```
   Decision file lands at `~/anvil-vault/30-decisions/<topic>.<NNNN>-<slug>.md` (MADR-conformant per `docs/system-design/knowledge-base.md`). Body is one paragraph: what was considered, why rejected. If no, no artifact.
3. **Paused** — user wants to think more. No artifact. If the source was an inbox item, it stays as-is for later resumption.

---

## What this skill does NOT do

- Does not design solutions or list approaches. That is `anvil:writing-plan`.
- Does not create milestones inline. It hands off to `anvil:writing-milestone` and resumes after.
- Does not run research. It can flag the need for it.
- Does not persist pre-mortem or working-backwards headline. Validation tools, not specification content.
