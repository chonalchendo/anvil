---
title: "Anvil: vault frontmatter schemas"
project: anvil
created: 2026-04-27
updated: 2026-04-27
related: "[[system-design.anvil]]"
---

# Anvil: Vault Frontmatter Schemas

Reference catalog: every vault artifact type with concrete YAML examples and field rationale. This document supports [`system-design.md`](system-design.md); the architectural shape lives there, the per-type schemas live here.

Every artifact carries a small set of universal fields so a single Bases query can list everything regardless of type: `id`, `type`, `title`, `created`, `updated`, `tags`, `status`, `aliases`. On top of that, each type extends with type-specific fields. JSON Schemas for each type ship at `anvil/schemas/*.schema.json` and are validated in CI via `check-jsonschema`.

Use Obsidian Properties-compliant types only (Text, List, Number, Checkbox, Date, Date & time). No nested objects in top-level YAML except where unavoidable (`sweep.metrics`, document `revisions`) — Obsidian's Properties UI cannot edit nested objects and several plugins choke on them.

The schemas below are starter shapes. The CI validation is the load-bearing part — drift in any field across notes is the documented failure mode at scale, and schema-validated PRs prevent it.

## `product-design` — the project's vision (one per project)

```yaml
---
type: product-design
title: "Payment Service: product design"
project: payment-service
created: 2026-03-01
updated: 2026-04-26
status: active                # draft | active | superseded | retired
tags: [domain/payments, type/product-design]
target_users:
  - "Internal services needing payment capture"
  - "External merchants integrating via API"
problem_statement: "Centralize payment processing across all customer-facing surfaces"
success_metrics:
  - "99.9% payment success rate"
  - "<200ms p99 latency for capture endpoint"
  - "Idempotent retries handled correctly under load"
goals:
  - "Users feel the tool is on their side"
  - "First plan in ≤30 minutes for a fresh project"
constraints:
  - "Fixed time, variable scope (Shape Up)"
  - "v0.1 ships in 6 weeks"
appetite: "big-batch"          # small-batch | big-batch | explicit duration
risks:
  - "Subprocess streaming buffer overflow on long tool-result lines"
  - "Companion-pack drift if Superpowers reshapes its skills"
out_of_scope:
  - "Subscription billing (separate service)"
  - "Refund automation (manual for v1)"
milestones:
  - "[[milestone.payments.m1-capture-baseline]]"
  - "[[milestone.payments.m2-idempotency]]"
  - "[[milestone.payments.m3-multi-currency]]"
related:
  - "[[system-design.payments]]"
  - "[[decision.payments.0001-stripe-vs-self-host]]"
revisions:
  - { date: 2026-03-01, change: "Initial draft" }
  - { date: 2026-03-15, change: "Added multi-currency goal" }
  - { date: 2026-04-26, change: "Removed automated refunds from v1 scope" }
---

## What we're building
...
## Who it's for
...
## Why it matters
...
## Goals
*(outcome-shaped, qualitative; distinct from success metrics below)*
...
## Constraints & appetite
*(Shape Up — fixed time, variable scope; explicit time box)*
...
## What success looks like
*(measurable metrics)*
...
## Approach
*(fat-marker sketch — broad strokes only, no architecture)*
...
## What's deliberately out of scope
...
## Risks, rabbit holes, open questions
...
## Milestones
This product is delivered through these structural milestones:
- [[milestone.payments.m1-capture-baseline]] — basic capture flow
- ...
```

Stored at `~/anvil-vault/05-projects/<project>/product-design.md`. One per project. The new `goals` field captures outcome-shaped statements distinct from `success_metrics` (which must be measurable). The `constraints`, `appetite`, and `approach` fields adopt Shape Up framing — constraints are usually fixed-time, scope is variable, and the approach is a fat-marker sketch (broad strokes only, no architecture or tech choices). The `out_of_scope` field is critical and almost always missing in real-world docs — it's the explicit negative space that prevents invisible scope creep. The `revisions` array is one of the few places inline objects are justified; product designs evolve, and capturing the *narrative reason* alongside the dated change is more useful than relying on git diff alone. Status enum is narrower than the issue/plan enums because product designs don't pause or abandon; they're either the current vision, a draft of one, superseded by a newer version, or the project is retired.

## `system-design` — the architectural shape (one per project)

```yaml
---
type: system-design
title: "Payment Service: system design"
project: payment-service
created: 2026-03-05
updated: 2026-04-26
status: active                # draft | active | superseded | retired
tags: [domain/payments, type/system-design]
product_design: "[[product-design.payments]]"
tech_stack:
  language: python
  framework: fastapi
  database: postgres
  message_bus: redpanda
  deployment: kubernetes
key_invariants:
  - "Every payment attempt produces exactly one ledger entry"
  - "Idempotency keys expire after 24h"
  - "All amounts stored as integer cents in base currency"
authorized_decisions:
  - "[[decision.payments.0001-stripe-vs-self-host]]"
  - "[[decision.payments.0003-postgres-for-ledger]]"
  - "[[decision.payments.0007-idempotency-via-postgres]]"
boundary_diagrams: ["[[assets.payments.context-diagram]]"]
risks:
  - "Provider rate-limit cascades during a payment-spike window"
  - "Ledger contention under write-heavy retries"
revisions:
  - { date: 2026-03-05, change: "Initial draft after architecture spike" }
  - { date: 2026-04-26, change: "Added idempotency design" }
---

## Architectural overview
...
## Components and responsibilities
...
## Data flow
...
## Boundaries and integration points
...
## Key invariants
...
## Risks
...
## Why this shape
References the decisions that led here: [[decision.payments.0001-stripe-vs-self-host]] explains the build-vs-buy choice...
```

Stored at `~/anvil-vault/05-projects/<project>/system-design.md`. References the product design it implements. `key_invariants` is the load-bearing field — these are the things that must always be true about the system, listed explicitly so the agent can verify against them during planning and review. `authorized_decisions` closes the provenance chain at the system level: every meaningful architectural choice traces to a decision artifact, so the system design is justified rather than asserted.

The body contains mermaid diagrams as core content — context, component, and data flow diagrams are first-class parts of the document, not appendices. The `writing-system-design` skill produces these inline. Mermaid renders in Obsidian and GitHub natively; the diagram source is plain text, diffable, and survives plugin churn.

When the system design changes meaningfully (e.g., migrating from Postgres to CockroachDB), the change is captured both as a revision entry *and* as a new decision. The revision is the changelog; the decision is the reasoning. Both have value.

## `milestone` — structural component of the product

```yaml
---
type: milestone
title: "M3: OAuth provider integration"
project: payment-service
created: 2026-04-01
updated: 2026-04-26
status: in-progress           # planned | in-progress | done | abandoned
target_date: 2026-05-15
horizon: month
tags: [domain/auth, type/milestone]
product_design: "[[product-design.payments]]"
authorized_by:
  - "[[decision.auth.0007-use-jwt]]"
predecessors:
  - "[[milestone.auth.m2-session-management]]"
successors:
  - "[[milestone.auth.m4-mfa]]"
plans:
  - "[[plan.auth.q2-rollout]]"
issues:
  - "[[issue.auth.add-oauth-provider]]"
  - "[[issue.auth.token-refresh-flow]]"
acceptance:
  - "Users can log in via Google and GitHub"
  - "Tokens refresh automatically when expired"
  - "OAuth provider config is per-environment"
risks:
  - "Provider rate limits during peak signups"
related: ["[[learning.auth.token-storage-patterns]]"]
---

## What this milestone delivers
...
## What it depends on
...
## How we'll know it's done
...
```

Stored at `~/anvil-vault/85-milestones/<project>.<slug>.md`. Milestones are first-class artifacts because they outlive the plans that execute them, accumulate learnings and decisions specific to their delivery, and form the structural backbone that connects product design to operational work.

The `predecessors` / `successors` fields make the milestone graph explicit and queryable — "what's blocking M4?" is `file.predecessors` walked back; "what depends on M3?" is `file.successors` walked forward. Plans reference milestones (via wikilink), not the other way around — a milestone may span multiple plans across quarters, and the plan completes while the milestone continues.

## `learning` — distilled facts, debugging insights, gotchas

```yaml
---
type: learning
title: "CREATE INDEX CONCURRENTLY hangs while autovacuum runs"
created: 2026-04-26
updated: 2026-04-26
status: verified              # draft | verified | stale | retracted
diataxis: explanation         # tutorial | how-to | reference | explanation
confidence: high              # low | medium | high (mandatory; agents must distinguish hypothesis from fact)
tags: [domain/postgres, activity/debugging, pattern/concurrency, type/learning]
aliases: ["CIC autovacuum lock"]
sources:
  - "https://www.postgresql.org/docs/16/sql-createindex.html"
  - "[[thread.indexing.cic-hang-investigation]]"
related: ["[[learning.postgres.snapshot-too-old]]"]
parents: ["[[plan.indexing.q2-rollout]]"]
---
```

`diataxis` (Diátaxis framework) lets agents retrieve the right shape for the task: "show me a how-to for X" vs "show me an explanation of Y." `confidence` is mandatory — without it, agents treat hypothesis and verified fact identically.

## `decision` — MADR v4 conformant ADR

```yaml
---
type: decision
title: "Use JWT (RS256) for service-to-service auth"
status: accepted              # MADR enum: proposed | rejected | accepted | deprecated | superseded
date: 2026-04-26              # MADR: date last updated
created: 2026-03-12
updated: 2026-04-26
decision-makers: ["@alice", "@bob"]
consulted: ["@security-team"]
informed: ["@platform-eng"]
tags: [domain/auth, pattern/auth, type/decision]
supersedes: []
superseded_by: null
related: ["[[decision.auth.0003-mtls-internal]]"]
evidence: ["[[thread.auth.spike-2026-03]]"]
---

## Context and Problem Statement
...
## Decision Drivers
...
## Considered Options
...
## Decision Outcome
Chosen option: "JWT (RS256)", because ...
### Consequences
...
### Confirmation
...
```

Hyphenated MADR keys (`decision-makers`, `superseded_by`) are kept verbatim for ecosystem compatibility (ADR-Manager, structured-madr validators). MADR v4 moved superseded-by linkage out of the status string into a structured field — the schema follows that. Filenames use the MADR `nnnn-` prefix: `30-decisions/auth.0007-use-jwt.md`.

## `skill` — Anthropic SKILL.md (allow-list strict)

```yaml
# prettier-ignore
---
name: sqlmesh-best-practices
description: Use when working with sqlmesh, dbt models, incremental data pipelines, or asks about model dependencies, audits, or virtual environments. Do NOT use for general SQL questions or non-sqlmesh data tools.
license: MIT
allowed-tools: [Read, Edit, Bash, Grep]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: sqlmesh-best-practices
  vault_type: skill
  skill_type: knowledge          # workflow | knowledge
  side: user                     # meta | design | execution | user
  created: 2026-04-26
  updated: 2026-09-15
  tags: [domain/sqlmesh, pattern/data-pipelines, type/skill]
  diataxis: how-to
  authored_via: researching-domain        # which meta-skill produced it
  refreshed_via: synthesizing-knowledge-skill   # how it was last updated
  confidence: high                # low | medium | high
  status: experience-validated    # from-research-only | in-use | experience-validated | deprecated
  source_learnings:
    - "[[learning.sqlmesh.macro-rendering-order]]"
    - "[[learning.sqlmesh.audit-pre-vs-post]]"
    - "[[learning.sqlmesh.virtual-environment-gotcha]]"
  related: ["[[decision.data.0011-sqlmesh-vs-dbt]]"]
---
```

**Critical rules**: top-level keys are strictly `name | description | license | allowed-tools | metadata | compatibility` per Anthropic's importer. Anything else fails validation. All Anvil-specific metadata nests under `metadata:`. The `# prettier-ignore` directive above the frontmatter delimiter is mandatory — Prettier reformatting single-line descriptions to multi-line YAML breaks Claude Code registration silently.

**Validator constraints**: `description` ≤ 1024 chars (validator hard limit) but ≤ 250 chars practical (Claude Code listing truncation, issue #881). `name` ≤ 64 chars matching `^[a-z0-9-]+$` with no consecutive/leading/trailing hyphens; never contains the reserved words `claude` or `anthropic`. No XML angle brackets anywhere. Skills are folders (`40-skills/<n>/SKILL.md`) per the spec — non-negotiable. CI runs `check-jsonschema`, Anthropic's `quick_validate.py`, plus Anvil's body-length, ALL-CAPS proliferation, and namespace-handoff checks on every PR.

**Provenance fields** make the skill's lifecycle legible:
- `authored_via` records the meta-skill that produced the first draft (`extracting-skill-from-session`, `researching-domain`, or hand-authored).
- `refreshed_via` records the meta-skill used for the most recent update.
- `confidence` is mandatory: `low` (research-only, untested), `medium` (some real use), `high` (experience-validated). Agents weight skill content by confidence when synthesizing.
- `status` tracks lifecycle stage: `from-research-only` (just researched, not yet used), `in-use` (being applied, accumulating learnings), `experience-validated` (refined by real use, learnings incorporated), `deprecated` (superseded or wrong; left for provenance).
- `source_learnings` links to the vault learnings that the skill body distills. The skill body references rather than repeats — `[[learning.sqlmesh.macro-rendering-order]]` rather than the full content. When learnings update, the skill is a candidate for refresh.

## `sweep` — coordinated batch of changes across the codebase

```yaml
---
type: sweep
title: "Migrate all services from log4j 1.x to logback"
created: 2026-04-10
updated: 2026-04-26
status: in-progress           # planned | in-progress | merged | abandoned
breaking: false
scope: backend                # matches Conventional Commits scope
tags: [domain/jvm, activity/refactor, pattern/observability, type/sweep]
driver: "[[decision.logging.0009-mandate-logback]]"
plan: "[[plan.logging.q2-cleanup]]"
target_repos: [api-server, worker, gateway]
prs: ["org/api-server#1442", "org/worker#883"]
metrics:
  files_touched: 217
  lines_added: 1804
  lines_removed: 2913
  services_migrated: 7
  services_remaining: 2
related: ["[[learning.logback.async-appender-deadlock]]"]
parents: ["[[plan.logging.q2-cleanup]]"]
---
```

`scope` matches the commit-message scope, keeping git history and vault aligned. `metrics` is one of the few places a nested object is justified — Bases queries it via `formula.metrics.files_touched`; you'll edit metrics rarely enough that source-mode editing is acceptable.

## `thread` — investigation/exploration unit

```yaml
---
type: thread
title: "Why does CREATE INDEX CONCURRENTLY hang on a busy table?"
created: 2026-04-22
updated: 2026-04-23
status: closed                # open | paused | closed | abandoned
opened: 2026-04-22
closed: 2026-04-23
tags: [domain/postgres, activity/debugging, type/thread]
diataxis: explanation
question: "Why does CIC hang when an autovacuum is running?"
hypothesis:
  - "Autovacuum holds a conflicting ShareUpdateExclusiveLock"
  - "Long-running transaction blocks the snapshot wait phase"
resolution: "Hypothesis 2 confirmed; documented in [[learning.postgres.cic-snapshot-wait]]"
participants: ["@alice", "claude-sonnet-4.5"]
outcome:
  - "[[learning.postgres.cic-snapshot-wait]]"
  - "[[issue.platform.cic-monitoring-gap]]"
related: ["[[thread.indexing.cic-rollout-strategy]]"]
parents: ["[[sweep.indexing.cic-rollout]]"]
---
```

The `question / hypothesis / resolution` triple makes threads re-entrant across days — an agent picking up the thread tomorrow has the framing already stated. `participants` lists humans and AI agents because attribution matters for trust. `outcome` links to the durable artifacts the thread produced; the thread itself is ephemeral.

## `issue` — knowledge slice of a tracked problem

```yaml
---
type: issue
title: "CIC monitoring missing: no alert when index build hangs"
created: 2026-04-23
updated: 2026-04-26
status: external              # external | learning-only | resolved
severity: medium
external_ref: "JIRA-PLAT-4421"
external_url: "https://company.atlassian.net/browse/PLAT-4421"
tags: [domain/observability, pattern/observability, type/issue]
discovered_in: "[[thread.indexing.cic-hang-investigation]]"
learnings:
  - "[[learning.postgres.cic-snapshot-wait]]"
  - "[[learning.monitoring.lock-wait-alert-gap]]"
related: ["[[decision.monitoring.0011-stack-choice]]"]
---
```

The narrowed status enum is deliberate. Per the design's hard constraint, the vault is not the issue tracker. `external` means open in Linear/Jira/etc.; `learning-only` means we never opened a ticket; `resolved` means lessons captured, ticket closed. This is *different* from the operational issue files in `~/.anvil/projects/<n>/issues/`, which are the actual work items with full workflow state. The vault `issue` is a knowledge node for attaching learnings; the operational issue is the work backlog. Two distinct artifacts that happen to share a name.

## `plan` — forward-looking roadmap

```yaml
---
type: plan
title: "Q2 logging cleanup: log4j → logback across all services"
created: 2026-04-01
updated: 2026-04-26
status: active                # draft | active | paused | done | abandoned
horizon: quarter              # week | sprint | month | quarter | year | open
target_date: 2026-06-30
owner: "@alice"
stakeholders: ["@platform", "@security"]
tags: [domain/jvm, pattern/observability, type/plan]
authorized_by:
  - "[[decision.logging.0009-mandate-logback]]"
objectives:
  - "All production services off log4j 1.x"
  - "Centralized async appender configured per service"
milestones:
  - "[[milestone.logging.m1-baseline-inventory]]"
  - "[[milestone.logging.m2-pilot-migration]]"
  - "[[milestone.logging.m3-bulk-rollout]]"
sweeps:
  - "[[sweep.logging.log4j-to-logback-migration]]"
risks:
  - "Async appender deadlocks under heavy load (see [[learning.logback.async-appender-deadlock]])"
children: ["[[sweep.logging.log4j-to-logback-migration]]"]
---
```

`horizon` lets a Bases query answer "what are we committed to this quarter" without scanning bodies. `authorized_by` and `milestones` together close the **product-design → milestone → plan → sweep → commit** provenance chain — every commit's sweep references the plan, every plan references the milestones it serves and the decisions that authorized it, every milestone references the product design it realizes. An agent can mechanically trace any code change back to the project's purpose.

Milestones are wikilink references, not inline objects. The plan completes; the milestones it served continue to live in `85-milestones/` and accumulate further work.

## `transcript` and `session` — AI-generated session output

```yaml
---
type: transcript              # or: session (for human-distilled summaries)
title: "Postgres advisory locks debugging session"
source: claude-code           # claude-code | chatgpt | claude-web | cursor | continue
session_id: abc-123-def
created: 2026-04-26T14:32:00
status: raw                   # raw → triaged → distilled → archived
project: "[[plan.indexing.q2-rollout]]"
tags: [domain/postgres, activity/debugging, type/transcript]
distilled_to: []              # populated when promoted; links to learnings/decisions
retention_until: 2026-05-26
---
```

Transcripts are the high-volume artifact. They live in `10-sessions/raw/` with `status: raw` and a 30-day `retention_until` timestamp. When the user distills insights from a session, those become `learning` or `decision` artifacts in the proper folders, and the source transcript flips to `status: distilled` and moves to `10-sessions/distilled/` for permanent provenance retention.
