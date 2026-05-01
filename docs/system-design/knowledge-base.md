---
title: "Anvil system design — knowledge base"
tags: [domain/dev-tools, type/system-design-shard]
---

## Knowledge base (Obsidian vault)

**Two locations, separated by lifecycle:**
- `~/anvil-vault/` — knowledge artifacts (Markdown, git-versioned, opened in Obsidian)
- `~/.anvil/` — operational state that churns per-command (issue state, briefings, build cache, telemetry, per-project state keyed by git remote)

**Vault structure (type-first, flat-within-type, PARA-style numeric prefixes):**

```
~/anvil-vault/
├── AGENTS.md               # ≤5k always-on layer
├── CLAUDE.md               # symlink to AGENTS.md
├── 00-inbox/               # human capture only; agents never write
├── 05-projects/<project>/  # product-design.md + system-design.md
├── 10-sessions/{raw,distilled}/
├── 20-learnings/           # flat, topic-prefixed (Dendron-style)
├── 30-decisions/           # MADR-conformant ADRs
├── 40-skills/<skill>/      # SKILL.md visible; references/scripts/assets/ hidden
├── 50-sweeps/
├── 60-threads/
├── 70-issues/              # work items (single source of truth)
├── 80-plans/               # canonical; worktrees read from here
├── 85-milestones/          # bridges design and execution
├── 90-moc/dashboards/      # static MOCs + .base files
├── 99-archive/
├── _meta/                  # tag-conventions, frontmatter-schema, retention
└── schemas/                # JSON Schemas for CI validation
```

**Key conventions:**
- Project is a frontmatter field, NOT a folder (cross-project distillation requires this).
- Typed frontmatter schemas validated by JSON Schema in CI. Schemas + tag taxonomy live in [`vault-schemas.md`](../vault-schemas.md). Validation is non-negotiable from v0.1 — frontmatter drift is the documented #2 failure mode at scale and CI is the only effective prevention.
- 50-note backpressure rule on `00-inbox/` and `10-sessions/raw/` to prevent write-only-vault syndrome.
- Wikilink-based provenance: product-design → milestone → plan → sweep → issue → commit.

**Workflow stage → vault mapping:**

| Stage | Location | Lifecycle | Trail on promotion |
|---|---|---|---|
| Inbox | `00-inbox/` | 14d demote, 30d archive. Backpressure at 50. | Promoted file deleted (low-signal capture isn't worth provenance). |
| Design | `05-projects/<project>/{product,system}-design.md` | Long-lived; updated as understanding evolves. | Authorises milestones via wikilink. |
| Milestone | `85-milestones/<project>.<slug>.md` | Lives until shipped, then `status: done`. | Authorises plans via wikilink. |
| Issue | `70-issues/<project>.<slug>.md` | Single source of truth: criteria, severity, status. | Authorises plan; receives learning links on review. |
| Plan | `80-plans/<project>.<slug>.md` | **Canonical.** Worktrees read from this path. | References issue; `status: done` on review approval. |
| Session | `10-sessions/raw/<date>.<worktree>.md` | Auto-written. 50-note backpressure. | Insights → learnings; transcript → `distilled/`. |
| Learning | `20-learnings/<topic>.<slug>.md` | `status: verified \| stale \| retracted`. | Backlinks from issues, plans, decisions. |
| Decision | `30-decisions/<topic>.<NNNN>-<slug>.md` | MADR. `proposed \| accepted \| deprecated \| superseded`. | Authorises plans and system designs. |
| Sweep | `50-sweeps/<slug>.md` | Cross-cutting work. | Closes the decision → plan → sweep → commit chain. |
