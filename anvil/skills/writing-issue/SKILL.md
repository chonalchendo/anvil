---
name: writing-issue
description: "Use when a problem worth tracking surfaces. Triggers: 'open an issue for X', 'track this as an issue', 'should we build X', 'promote inbox item to issue'. Not for raw capture (capturing-inbox) or implementation (completing-issue)."
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

**If you can name the goal — a one-sentence definition of done — in one breath, it's an issue.** Use this skill when the entry is decisive (problem + goal + milestone hint) OR when a fuzzy thought is ready to be pressure-tested into one. Inbox-first is NOT required when the entry is already shaped — route here directly.

Wrong-choice example: user is dumping a half-formed thought with no nameable goal and no clear definition of done. That's an inbox item — hand off to `capturing-inbox` and resume here later via `anvil promote` if the thought sharpens.

## Iron Law

**NO ISSUE FILE LANDS WITHOUT AN EXPLICIT MILESTONE LINK.**

If no milestone fits, the workflow stops at Phase 2 and offers two exits: log a `decision` artifact with `status: rejected`, or hand off to `writing-milestone` and resume here once the milestone exists. There is no `--no-milestone` escape hatch.

## When this skill runs

- A problem worth tracking has surfaced — inbox item, ad-hoc message, or direct request.
- The user wants to think about whether to build something, OR already knows.
- A milestone exists for the target project, OR you are willing to create one mid-flight.

## When not to use

- The user is dumping a thought without engagement → `capturing-inbox`.
- You need to implement the issue → `completing-issue`.
- Editing existing issue frontmatter only (status flip, tag) → a direct `anvil set` call.

---

## Autonomous mode (unattended runs)

When the caller declares an unattended run (e.g. an overnight self-test loop), resolve every human-confirm in this skill from the rubric instead of asking — the morning PR review is the gate. The Iron Law still holds: no issue lands without a milestone link.

- **Severity (Phase 4):** pick from the rubric; do not confirm.
- **Convergence (Phase 1):** skip — an unattended self-test finding already carries a problem, a goal, and a reproduction, so there is nothing fuzzy to converge. If a finding is genuinely fuzzy (no nameable goal), do not converge solo: capture it as an `inbox` item and stop.
- **Milestone-fit (Phase 2):** auto file-or-skip. A fitting milestone → file. No fit → do not prompt and do not invent one: capture as `inbox` and stop.

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
  - a goal — a one-sentence terminal predicate naming what "done" means,
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
- **No match, idea reshapes the system** → stop and offer to hand off to `writing-milestone`. Resume `writing-issue` after the milestone exists. **REQUIRED SUB-SKILL:** Use writing-milestone.

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

### Classify the kind (drives body content)

Classify the issue into exactly one kind before composing the body — it decides what the body must carry. Each kind has different load-bearing content, and forcing the wrong shape (e.g. a `reproduction_anchor` on a feature) is a category error.

- **bug** — something concrete is broken today and a command reproduces it.
- **feature** — a new capability; nothing exists to reproduce yet.
- **refactor** — an internal shape change behind a held invariant.
- **docs** — a documentation gap for a named audience.

**REQUIRED REFERENCE:** Use skills/writing-issue/references/<kind>.md

The reference owns the kind-specific body shape (and, for bugs, the `reproduction_anchor`). The phases above and the CLI mechanics below are kind-agnostic.

Before calling the CLI, **propose severity using the rubric** above, then confirm with the user. The agent does the first-pass classification rather than defaulting to `medium`; severity is required by the schema and gates triage queries.

List the existing `domain/` taxonomy so you reuse a value the user has already introduced rather than coining a near-duplicate:

```bash
anvil tags list --source used --prefix domain/ --json
```

Pick the closest existing value if one fits; only invent a new one if no existing value matches. The CLI will reject an unrecognised value unless you pass `--allow-new-facet=domain` — verbosity is intentional friction.

When promoting an inbox item, pass `--tags` on the `anvil promote <id> --as issue` call after consulting the same list.

Compose the **goal** first: one sentence, ≤120 chars, naming what "done" means — the issue's terminal predicate. It is required (`--goal`) and gates the claim later. Keep it a predicate ("inbox no longer drops items on concurrent writes"), not a task list.

Author the body up front and pass it to `create` via `--body-file` (or `--body -` for piped stdin). `create` validates the frontmatter AND body (required H2s, wikilink targets) and rolls back the write on failure — no separate `anvil validate` step. The `## Verification` block uses fenced bash; see `docs/issue-spec.md` for the full format spec.

````bash
cat > /tmp/issue-body.md <<'EOF'
## Problem
<one paragraph from convergence (fuzzy) or the stated problem (decisive)>

## Non-goals
- <from Phase 3 smallest-viable or stated up front>

## Verification

### Direct (unit/integration)
```bash
<shell command — exit 0 = pass>
```

### Indirect (live smoke)
```bash
<shell command with predicate baked in — grep -q "X", jq -r .field, [ ... = ... ]>
```

## Links
- [[milestone.<project>.<slug>]]
EOF

anvil create issue --title "<title>" --goal "<one-sentence definition of done>" --tags domain/<x> --body-file /tmp/issue-body.md --json
````

An optional `## Acceptance criteria` prose checklist may follow `## Problem` when an unambiguous bulleted list aids the implementer — but it is no longer required, and the binary gate is `## Verification`, not AC.

Capture `id` and `path` from the JSON output. The file lands at `~/anvil-vault/70-issues/<project>.<slug>.md`.

Set typed frontmatter slots (these are still post-create — typed setters live outside the body):

```bash
anvil set issue <id> milestone "[[milestone.<project>.<slug>]]"
anvil set issue <id> severity <low|medium|high|critical>
```

`acceptance[]` is **optional**. Add bullets only when a prose checklist genuinely aids the implementer beyond `goal:` + `## Verification`; most issues need none. When you do, run one `--add` per criterion:

```bash
anvil set issue <id> acceptance --add "<criterion>"
```

Positional values on array fields error with `field_is_array`; use `--add VALUE` to append and `--remove INDEX` to delete.

Required body sections (enforced by `create`):

- `## Problem` — one paragraph from convergence (fuzzy) or the stated problem (decisive).
- `## Non-goals` — from Phase 3 smallest-viable (fuzzy) or stated up front (decisive).
- `## Verification` — operational checks in fenced bash blocks (full spec: `docs/issue-spec.md`). Two subsections, both required:
  - `### Direct` — fenced `bash` block with ≥1 line. Each line must exit 0. Typically unit/integration tests run against the dev tree.
  - `### Indirect` — fenced `bash` block with ≥1 line. Each line must exit 0. Live invocations against the built/installed/served artifact; bake the predicate into the command (`grep -q "X"`, `jq -r .field`, `[ ... = ... ]`). `completing-issue` re-runs these against the installed binary in its Phase 4 build gate — they catch behavioral gaps the Direct checks can't see. Each predicate must exercise behaviour and assert on observed output or side-effects — presence-only anti-patterns and the doc/skill-only carve-out are enumerated in `docs/issue-spec.md#indirect-live-smoke`.
- `## Links` — to milestone, design docs, related issues. Use `[[wikilink]]` form. Targets must resolve (the file must exist) or `create` rejects.

`anvil validate <path>` remains useful as a re-check after edits (e.g. after `anvil set ... acceptance --add`), but it is **not** required after `create` when the body was supplied via `--body-file` / `--body -`.

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

1. **`issue` created** — file exists, validates, milestone link set. Hand off to `completing-issue` for implementation.
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

- Does not implement the issue. That is `completing-issue`.
- Does not create milestones inline. It hands off to `writing-milestone` and resumes after.
- Does not run research. It can flag the need for it.
- Does not persist pre-mortem or working-backwards headline. Validation tools, not specification content.
