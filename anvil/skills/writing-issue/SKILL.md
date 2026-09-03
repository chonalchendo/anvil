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
  updated: 2026-09-03
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

**NO ISSUE FILE LANDS WITHOUT AN EXPLICIT MILESTONE LINK.** If no milestone fits, the workflow stops at Phase 2 and offers two exits: log a `decision` artifact with `status: rejected`, or hand off to `writing-milestone` and resume here once the milestone exists. There is no `--no-milestone` escape hatch.

## When this skill runs / not

- **Runs:** a problem worth tracking surfaced (inbox item, ad-hoc message, direct request); the user wants to weigh whether to build something, or already knows; a milestone exists for the project, or you are willing to create one mid-flight.
- **Not:** the user is dumping a thought without engagement (`capturing-inbox`); you need to implement the issue (`completing-issue`); editing existing issue frontmatter only (a direct `anvil set` call).

**REQUIRED REFERENCE:** Use skills/writing-issue/references/autonomous-mode.md when the caller declares an unattended run — resolves every human-confirm below from the severity rubric instead of asking. For bulk decisive-path authoring, dispatch to `anvil-issue-author` (cheaper-model subagent, `dispatch-single.md`) instead of running this skill directly.

## Severity rubric

Anchor severity on **blast-radius × workaround-cost**:

- `critical` — corrupts data, breaks the schema, or makes `anvil` itself unusable. No workaround.
- `high` — blocks a documented workflow; agent or human must context-switch around it. Workaround exists but is costly enough that fixing-now is cheaper than working-around-twice.
- `medium` — adds friction (time, tokens, round-trips) but doesn't block; clear cheap workaround.
- `low` — polish, cosmetic, missing affordance; costs little to live with.

## Phase 0 — Entry detection

Classify the user's first message before doing anything else — the classification chooses which phases run.

- **Decisive** when the message names all three of a problem statement, a goal (one-sentence terminal predicate), and a milestone reference (an id, or a phrase mapping to exactly one existing milestone under `~/anvil-vault/85-milestones/`, filtered by `project`; two-or-more plausible matches count as fuzzy).
- **Fuzzy** otherwise ("should we build X", "is this worth doing", or any message missing one of the three signals).
- **Tie-break:** when in doubt, run convergence — misclassifying decisive→fuzzy costs one round-trip; fuzzy→decisive ships a thin issue.

Decisive → skip to Phase 2. Fuzzy → continue to Phase 1. No artifact or chat output beyond the routing decision.

## Phase 1 — Convergence (fuzzy path only)

Goal: a one-sentence shared understanding of what is being proposed, so Phase 3's pressure-test stress-tests the user's idea, not your interpretation. Restate it in one sentence, ask "did I get this right?", one clarifying question at a time (multiple-choice preferred), stop only on explicit confirmation ("sure, whatever" doesn't count). Output: a `Problem` sentence and a `Proposal` sentence held in chat for Phase 4.

## Phase 2 — Milestone-fit gate (always; Iron Law)

Compare the converged/stated proposal against `~/anvil-vault/85-milestones/`, filtered by `project`.

- **Match found** → record the milestone id; continue to Phase 2b, then Phase 3 (fuzzy) or Phase 4 (decisive).
- **No match, idea is small or orthogonal** → offer two exits: (a) log a `decision` artifact with `status: rejected` — CLI sequence in the Terminal states reference below; (b) stop without an artifact (inbox source, if any, stays as-is).
- **No match, idea reshapes the system** → stop, offer to hand off. **REQUIRED SUB-SKILL:** Use writing-milestone. Resume here after the milestone exists.

Never skip the gate to issue creation. **Frame the fork, then recommend.** When the gate forks — log-vs-stop, build-now-vs-defer, this milestone vs that — make it legible in a few lines before recommending: name the options, state the tension, surface the rejected alternative *and why it fails*, give the one discriminating fact. Then recommend a single direction, don't hand back a bare menu. Stay silent on trivial choices; never manufacture tension.

## Phase 2b — Read the upstream design closure (always)

Issues authored against a milestone with no design read come out bare. Read the closure now, before drafting `## Problem`:

```bash
anvil show milestone <id> --body                     # the milestone's own objectives/non-goals
anvil show milestone <id> --links product-design --body; anvil show milestone <id> --links system-design --body
```

Hold whichever bodies resolve as authoring context — ground `## Problem` prose in real implementation constraints, sharpen `## Non-goals`, and note which subsystem governs for Phase 4b's link. **Read, don't copy**: no design paragraph gets pasted into the issue body. Neither link resolves → nothing to read; continue.

## Phase 3 — Pressure-test (fuzzy path only)

Three short frames (a paragraph or less each) against the converged proposal. Gate-side and discarded after passing, except `smallest-viable`, which persists as `## Non-goals`.

1. **Pre-mortem.** "Six months from now, this shipped but it was the wrong call — why?" 2–3 plausible failure reasons; a load-bearing one becomes a non-goal or kills the issue.
2. **Smallest viable version.** Thinnest cut that still delivers the win; what's explicitly out of scope. *Persists* into Phase 4 as `## Non-goals`.
3. **Working-backwards headline.** "We shipped X so users can Y." Boring, vague, or disconnected from a product-design success metric → return to Phase 1.

Skip a frame only when genuinely not applicable; record why in chat. A frame surfacing an unknown needing evidence → recommend research as a separate side task, don't block on it.

## Phase 3b — Surface prior learnings (always)

Before authoring, dispatch `anvil-learnings-researcher` via the Agent tool's `subagent_type` to pull what the vault already knows about this slice. **REQUIRED REFERENCE:** Use skills/writing-issue/references/learnings-dispatch.md for the `<work-context>` shape and how to fold findings into the issue.

## Phase 4 — Author the issue (always)

Classify the issue into exactly one kind before composing the body — bug (concrete + reproducible), feature (new capability), refactor (internal shape change, held invariant), docs (gap for a named audience). **REQUIRED REFERENCE:** Use skills/writing-issue/references/<kind>.md for the kind-specific body shape (and, for bugs, the `reproduction_anchor`).

- **Severity & domain tags**
  - Propose severity from the rubric above; confirm with the user. Don't default to `medium`.
  - Reuse an existing `domain/` value (`anvil tags list --source used --prefix domain/ --json`) over coining a near-duplicate — the CLI rejects an unrecognised value unless `--allow-new-facet=domain`. Promoting an inbox item: same check, then `--tags` on `anvil promote <id> --as issue`.
- **Goal** — one sentence, ≤120 chars, naming what "done" means (`--goal`, required, gates the later claim). Outcome, not mechanism: mechanism detail belongs in `## Problem`'s Direction part, not `goal:`/AC. A predicate, not a task list.
- **Feasibility gate**
  - If an AC or `## Verification` block prescribes a tool/command/behaviour as the mechanism, run that command in this environment before the issue lands; on failure, rewrite as an outcome or split a feasibility spike.
  - `create`/`promote --as issue` enforce this mechanically: every `### Direct`/`### Indirect` block actually runs, judged by exit status. An Indirect block that already passes is the failure — it can't discriminate fixed from broken.
  - Full verdict table, `set -e`/SIGPIPE rules: `docs/issue-spec.md`.
- **Draw verification from the governing contract, don't invent predicates** — identify it before authoring (`anvil list contract` → `anvil show contract <id> --body`; link recorded in Phase 4b) and write `### Direct`/`### Indirect` as its concrete instance. Every predicate, contract-drawn or not, satisfies the universal bars: same code path, exercise not presence (behaviour, never a source grep — except doc/skill-only changes, which grep the *built/installed* artifact), create the unmet condition first, anchor structurally, and the goal's own measure. Definitions and full predicate-writing rules: `docs/issue-spec.md`.
- **`## Problem` for a cold reader** — lead sentence, then bold-labelled parts (evidence, cause, direction, sequencing); enumerations are lists or tables, never paragraphs. **REQUIRED REFERENCE:** Use skills/writing-issue/references/problem-shape.md for the per-part shape and the cold-reader and glance tests.

Print the required skeleton, fill it, and pass via `--body-file` (`create` validates frontmatter + body and rolls back on failure — no separate `validate` step):

```bash
anvil create issue --show-template   # prints the required-H2 skeleton
anvil create issue --title "<title>" --description "<one-line preview>" \
  --goal "<one-sentence definition of done>" --tags domain/<x> \
  --body-file /tmp/issue-body.md --json
```

Required H2s (`create` rejects a body missing any): `## Problem`, `## Non-goals` (bulleted), `## Verification` (`### Direct` + `### Indirect`, fenced `bash` blocks — shape/rules in `docs/issue-spec.md`), `## Links` (`[[wikilink]]`, targets must resolve; `anvil hydrate` walks these links only for governing types — a sibling-issue link resolves but stays inert). `## Acceptance criteria` is optional, only when a bulleted checklist beats `goal:` + `## Verification` alone.

Capture `id`/`path` from the JSON output (`~/anvil-vault/70-issues/issue.<project>.NNNN.<slug>.md`), then set typed slots — bare positional values on array fields **replace** the array, use `--add`/`--remove VALUE_OR_INDEX`:

```bash
anvil set issue <id> milestone "[[milestone.<project>.<slug>]]"
anvil set issue <id> severity <low|medium|high|critical>
anvil set issue <id> acceptance --add "<criterion>"   # optional, one --add per criterion
```

## Phase 4b — Typed slots: contracts, system-design, dependencies, anchor

Link the governing context a worker loads at issue-start (`completing-issue` Phase 1) and a reviewer uses as a rubric (`reviewing-pr`), then set the slots the queue and claim gate read. Substitute real ids — `anvil link` refuses a target still carrying `<`, `>`, or whitespace.

- **Contract(s)** — `anvil list contract --json`; for each whose scope matches: `anvil link issue <issue-id> contract <contract-id>`.
- **System-design** — `anvil list system-design --json`; match on `project` equality: `anvil link issue <issue-id> system-design <project>`. This is the issue's governing spine edge that `completing-issue` walks to hydrate its box; make a missing link an explicit decision (attach it, or state "no design governs this slice") — never a silent skip. Don't invent a link to satisfy the check.
- **Dependencies** — one edge per issue the Sequencing line names: `anvil link issue <issue-id> issue <prereq-id> --relation depends_on` / `--relation blocks`. `anvil list issue --ready` reads only these typed edges — prose ordering is invisible to it.
- **Reproduction anchor** — bug kind only; shape lives in `references/bug.md`. Author one whenever a command can capture the failure; skipping it is a stated decision, never a silent default.

**REQUIRED REFERENCE:** Use skills/writing-issue/references/terminal-states.md for the `anvil transition` state machine and the three completion exits (issue created, decision/rejected, paused).

## What this skill does NOT do

- Does not implement the issue — hands off to `completing-issue`.
- Does not create milestones inline — hands off to `writing-milestone`, resumes after.
- Does not run research, only flag the need for it.
- Does not persist pre-mortem or working-backwards headline — validation tools, not specification content.
