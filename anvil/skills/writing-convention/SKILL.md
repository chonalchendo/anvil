---
name: writing-convention
description: "Use when authoring or sharpening a cross-project code/style convention — how you write Python, SQL, Terraform, or model data across every project. Triggers: 'write the X convention', 'capture the house style for X', 'these rules should be shared across projects'. Not for a project-specific boundary (writing-contract) or a point-in-time choice (a decision)."
license: MIT
allowed-tools: [Bash, Read, Edit, Write]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-convention
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-06-27
  updated: 2026-06-27
  tags: [type/skill, activity/convention]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Writing Convention

Workflow for authoring or sharpening a **convention** — a project-agnostic, tool/language-keyed code/style spec (`convention.python`, `convention.sql`, `convention.terraform`, `convention.data-modelling`). A convention is the single canonical source a contract or project doc *links*, never restates.

## The boundary: convention = content, skill = loader

A convention holds the **standing spec** — the rules themselves. It is not a behavioural loader. Where each neighbour sits:

- **decision** — *why/when* a rule changed (the changelog, with reversal triggers). Not the standing rule.
- **convention** — the standing cross-project spec. Authored once; the source of truth.
- **contract `## Code design`** — *links* the governing convention(s) plus this component's project-specific deltas.
- **skill** — a thin behavioural loader. A skill (including this one) points at a convention; it never forks the convention's content into its own body.

When you find yourself copying a convention's rules into a contract, a skill, or a project `CLAUDE.md`, stop — link `[[convention.<slug>]]` instead. Duplication is the drift this type exists to kill.

## Mode selection

**Author mode** — no convention exists for this tool/language yet. Decide before Phase 1; `anvil list convention` confirms.

**Update mode** — a convention exists; you are adding, sharpening, or removing a rule.

## Convention skeleton

Conventions are prose-flexible (the schema enforces frontmatter, not body sections), but a readable convention opens with scope then groups rules by concern:

```
## Scope

<One paragraph: what this governs, that it is the cross-project default, and that a
project CLAUDE.md/AGENTS.md may link it and record local deltas.>

## <Concern>

- <rule — terse, imperative, with a short example where the rule is non-obvious>.
```

Keep each rule one or two lines. A rule that needs a paragraph is usually two rules.

---

## Author mode

### Phase 1 — Discover layout

Read the project's CLAUDE.md / AGENTS.md to learn how the vault is invoked (the `anvil` CLI discovers its own paths — never hardcode a vault directory). Then:

```bash
anvil list convention            # confirm none exists for this tool/language
```

### Phase 2 — Source the rules

Ground the convention in real, observed rules, not invented ideals. Pull from existing per-repo `CLAUDE.md` / `docs/*-conventions.md` across projects and consolidate the overlap — the point is to author once what was drifting in N places. Strip project-specific examples so the spec stays project-agnostic.

### Phase 3 — Create

```bash
anvil create convention --slug <tool> --title "<Tool>" \
  --description "<one sentence — what house style this is>" \
  --body-file <path>
```

The id is `convention.<slug>` (project-agnostic — no `--project`). Validate, then promote:

```bash
anvil validate
anvil set convention convention.<slug> status active
```

If `anvil validate` reports `type/convention` as an unknown glossary tag (first convention in a fresh vault), register it once: `anvil tags add type/convention --desc "..."`.

---

## Update mode

```bash
anvil list convention --json     # find the id
anvil show convention convention.<slug> --body
```

Open the file directly and make the minimal edit — add, sharpen, or remove a rule. Bump `updated`:

```bash
anvil set convention convention.<slug> updated <today-iso>
anvil validate
```

A convention is a **mutable current-state doc**, not an append-only thread — edit in place so a reader gets the current rule in one read. Don't grow an in-doc changelog: git carries routine edit history. For a rule change that warrants a *why*-record (a reversal trigger, a rule that bit you), file a `decision --topic <slug>` linking `[[convention.<slug>]]` — the decision thread is the changelog, the convention is its rolled-up current-state view.

---

## Surfacing at write-time (optional, per-repo)

A convention only enforces if it reaches the agent *as it writes* the matching code. The contract rail (`anvil show contract <id> --links convention --body`, read by `completing-issue` and `reviewing-pr`) covers issue work; for editor-driven edits a project can add a `PreToolUse` hook that injects the convention directly.

The pattern: match the edited `file_path` by extension (`*.py`, `*.sql`, …) → inject `anvil show convention convention.<lang> --body` as `additionalContext` → dedup once per session via a sentinel file → `permissionDecision: defer` (never approve/deny). The hook is per-repo build infra — it may hardcode the repo's paths and extension map, so it lives in the project, not in this skill.

---

## Non-goals

- Project-specific boundaries — that is a contract (`writing-contract`), which *links* this convention.
- Machine enforcement (`anvil convention check`) — conventions are read by agents, not linted, in v0.1.
- Auto-generating project `CLAUDE.md` pointer blocks — link by hand.
