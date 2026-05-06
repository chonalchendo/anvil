---
title: "Anvil: vault frontmatter schemas"
project: anvil
created: 2026-04-27
updated: 2026-05-01
related:
  - "[[system-design.anvil]]"
  - "[[2026-05-01-vault-schemas-redesign-design]]"
---

# Anvil: Vault Frontmatter Schemas

Per-type reference for vault frontmatter. Rationale and the three rules that shape every schema below live in [`docs/superpowers/specs/2026-05-01-vault-schemas-redesign-design.md`](superpowers/specs/2026-05-01-vault-schemas-redesign-design.md). This file is reference, not narrative.

JSON Schemas ship at `schemas/*.schema.json` and are validated in CI via the `anvil validate` verb. Frontmatter must be Obsidian Properties-compliant — no nested objects at the top level except where unavoidable (`plan.tasks`, `plan.verification`).

## Universal fields

Implied on every type unless explicitly overridden:

- `type` — discriminator, matches the schema's `const`.
- `title` — display name.
- `created` / `updated` — ISO date.
- `status` — type-specific enum (listed below).
- `tags` — flat list. Per-type schemas may require minimum facets (see per-type sections). Values match `[a-z0-9-]+` (lowercase, hyphenated, ASCII) under a `<facet>/<value>` shape. The CLI gate (`anvil create`, `anvil set tags`, `anvil promote`) rejects values novel to the vault unless `--allow-new-facet=<facet>` is passed.
- `aliases` — Obsidian aliases.
- `related` — wikilink array of associative pointers.

## The spine

Structural edges, child → parent, typed scalars unless noted:

| Child | Slot | Parent | Cardinality |
|---|---|---|---|
| `system-design` | `product_design` | product-design | 1:1 |
| `milestone` | `product_design` | product-design | 1:1 |
| `milestone` | `system_design` | system-design | 1:1 |
| `issue` | `milestone` | milestone | 1:1 |
| `plan` | `issue` | issue | 1:1 |
| `sweep` | `plan` | plan | 1:1 |
| `inbox` | `promoted_to` | any | 0:1 |

Authorization edges (typed arrays on the child):

- `milestone.authorized_by: [decision...]`
- `system-design.authorized_by: [decision...]`
- `plan.authorized_by: [decision...]`

Decision-evolution links stay typed:

- `decision.supersedes: [decision...]`
- `decision.superseded_by: decision | null`

Everything else is `related`.

## Per-type schemas

### `inbox`

```yaml
type: inbox
status: raw | triaged | promoted | dropped
suggested_type: issue | design | learning | discard
suggested_project: <project> | null
promoted_to: "[[<artifact>]]" | null
```

### `product-design`

```yaml
type: product-design
project: <slug>
status: draft | active | superseded | retired
```

Body absorbs: target-users, problem statement, success metrics, goals, constraints, appetite, risks, out-of-scope, revisions.

### `system-design`

```yaml
type: system-design
project: <slug>
status: draft | active | superseded | retired
product_design: "[[product-design.<project>]]"
authorized_by: ["[[decision...]]"]
```

Body absorbs: tech stack, key invariants, risks, boundary diagrams, revisions. Mermaid diagrams stay first-class body content.

### `milestone`

```yaml
type: milestone
project: <slug>
status: planned | in-progress | done | abandoned
product_design: "[[product-design.<project>]]"
system_design: "[[system-design.<project>]]"
authorized_by: ["[[decision...]]"]
acceptance: ["criterion", ...]
```

Cut entirely: `target_date`, `horizon`, `ordinal`, `predecessors`, `successors`, `plans`, `issues`, `objectives`, `risks`. Milestones are structural, not scheduled. Done = all child issues `resolved`.

### `issue`

```yaml
type: issue
project: <slug>
status: open | in-progress | resolved | abandoned
severity: low | medium | high | critical
milestone: "[[milestone.<project>.<slug>]]"
external_ref: <string> | null
external_url: <url> | null
acceptance: ["criterion", ...]
```

Single source of truth. Knowledge attaches via the child side: `learning.related: [[issue.X]]`. No `learnings`, `discovered_in`, or `promoted_from` arrays on the issue.

Tags: required `domain/<x>`; `activity/<x>` and `pattern/<x>` optional.

### `plan`

```yaml
type: plan
id: <project>.<slug>
project: <slug>
status: draft | locked | in-progress | done | abandoned
plan_version: <integer>
issue: "[[issue.<project>.<slug>]]"
authorized_by: ["[[decision...]]"]
tasks:
  - id: T1
    title: ...
    kind: tdd | mechanical
    model: sonnet-4.6 | opus-4.7 | haiku-4.5    # optional
    effort: low | medium | high                  # optional
    files: [...]
    depends_on: [T<n>, ...]
    skills_to_load: [<skill-name>, ...]
    verify: <command>
    success_criteria: [...]
verification:
  pre_build:  [<command>, ...]
  post_build: [<command>, ...]
```

`task.model` / `task.effort` are present only when the task diverges from orchestrator defaults (sonnet-4.6 / medium). Defaults live in `~/.anvil/config.yaml`, not in plan frontmatter.

`task.skills_to_load` is load-bearing: the build orchestrator materializes `skills_to_load + always-on core` into each spawn's state dir, not all bundled skills.

Plan `milestone` is derived (`plan.issue → issue.milestone`); not stored.

Tags: required `domain/<x>`; `activity/<x>` and `pattern/<x>` optional.

### `decision`

```yaml
type: decision
status: proposed | rejected | accepted | deprecated | superseded
date: <iso-date>
supersedes: ["[[decision...]]"]
superseded_by: "[[decision...]]" | null
```

Body absorbs: decision-makers, consulted, informed, evidence. Filenames keep the MADR `nnnn-` numeric prefix in the slug: `30-decisions/<project>.<NNNN>-<slug>.md`.

Tags: required `domain/<x>` and `activity/<x>`; `pattern/<x>` optional.

### `learning`

```yaml
type: learning
status: draft | verified | stale | retracted
diataxis: tutorial | how-to | reference | explanation
confidence: low | medium | high
```

Body absorbs: sources. Cut entirely: `parents` (use `related`).

Tags: required `domain/<x>` and `activity/<x>`; `pattern/<x>` optional.

### `thread`

```yaml
type: thread
status: open | paused | closed | abandoned
diataxis: tutorial | how-to | reference | explanation
```

Body absorbs: question, hypothesis, resolution, participants. Cut entirely: `opened`, `closed` (universal `created` / `updated` cover this).

Tags: required `domain/<x>` and `activity/<x>`; `pattern/<x>` optional.

### `sweep`

```yaml
type: sweep
status: planned | in-progress | merged | abandoned
breaking: <bool>
scope: <commit-scope>
```

Body absorbs: target-repos, prs, metrics, driver. `breaking` and `scope` drive Conventional-Commits generation downstream.

### `transcript` / `session`

```yaml
type: transcript | session
source: claude-code | chatgpt | claude-web | cursor | continue
session_id: <id>
status: raw | triaged | distilled | archived
retention_until: <iso-date>
```

### `skill (bundled)` — `skills/<skill>/SKILL.md`

```yaml
---
name: <skill-name>
description: <≤1024 chars; ≤250 practical>
license: MIT                # optional
allowed-tools: [...]        # optional
compatibility: ...          # optional
---
```

Anthropic SKILL.md spec only. Bundled skills are validated by the agent CLI's loader, not by `anvil validate`.

### `skill (vault)` — `~/anvil-vault/40-skills/<skill>/SKILL.md`

```yaml
---
name: <skill-name>
description: ...
allowed-tools: [...]
metadata:
  vault_id: <slug>
  vault_type: skill
  skill_type: workflow | knowledge
  side: meta | design | execution | user
  status: from-research-only | in-use | experience-validated | deprecated
  diataxis: tutorial | how-to | reference | explanation
  confidence: low | medium | high
  authored_via: <meta-skill-name>
  refreshed_via: <meta-skill-name>
  related: ["[[learning...]]", ...]
---
```

User-authored. Anthropic spec at top level + Anvil `metadata:` block. Out of CLI surface for v0.1.

## IDs and naming

Slug-based across all artifacts. Wikilink form: `<type>.<project>.<slug>`; filename: `<project>.<slug>.md` within the type folder.

Examples:

- `[[milestone.anvil.cli-substrate]]` → `85-milestones/anvil.cli-substrate.md`
- `[[issue.anvil.fix-inbox-suggested-type]]` → `70-issues/anvil.fix-inbox-suggested-type.md`
- `[[plan.anvil.streaming-token-counter]]` → `80-plans/anvil.streaming-token-counter.md`
- `[[decision.anvil.0001-go-rewrite]]` → `30-decisions/anvil.0001-go-rewrite.md`

Two rules:

1. Slugs are immutable once allocated. Renaming the title doesn't rename the slug.
2. `anvil create` allocates the slug from the title at creation time, normalized (lowercase, hyphenated, ASCII).

The plan `id` collapses with the slug: `plan.id == <project>.<slug>`. Decisions keep their MADR numeric prefix in the slug.

## Folder structure

```
~/anvil-vault/
├── 00-inbox/
├── 05-projects/<project>/        # product-design.md + system-design.md
├── 10-sessions/{raw,distilled}/
├── 20-learnings/
├── 30-decisions/
├── 40-skills/<skill>/            # vault skills (user-authored)
├── 50-sweeps/
├── 60-threads/
├── 70-issues/                    # work items (single source of truth)
├── 80-plans/
├── 85-milestones/
├── 90-moc/dashboards/
├── 99-archive/
├── _meta/
└── schemas/
```

Bundled core skills stay outside the vault under `skills/<skill>/SKILL.md` in the Anvil source tree.
