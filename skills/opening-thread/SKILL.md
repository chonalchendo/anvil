---
name: opening-thread
description: "Use when the user explicitly commits to research now. Triggers: 'let's research X', 'open a thread on X', 'promote inbox <id> to a thread'. Not for passive captures (use capturing-inbox) or project-tied research with an existing issue."
license: MIT
allowed-tools: [Bash, Read, Edit]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: opening-thread
  vault_type: skill
  skill_type: workflow
  side: execution
  created: 2026-05-01
  updated: 2026-05-01
  tags: [type/skill, activity/research]
  diataxis: how-to
  authored_via: manual
  confidence: low
  status: in-use
---

# Opening Thread

Workflow for opening a research thread — the live workspace for cross-session inquiry. Threads sit in the **knowledge pipeline**, parallel to the build pipeline (inbox → issue → plan). They are the workspace; learnings are the durable output.

## When this skill runs

- The user explicitly commits to research now.
- A passive inbox entry is being promoted to a thread.
- A milestone may exist but the work is research, not project-bound.

## When not to use

- The user is dumping a thought without committing → `anvil:capturing-inbox`.
- Project-tied research with an existing issue → research happens in plan-execution context.
- The thread already exists and the user is resuming → no skill needed; sessions auto-bind to active thread.

---

## Phase 1 — Context

If promoting from inbox, read the entry:

```bash
anvil show inbox <inbox-id>
```

Read `_meta/glossary.md` (if present) to know existing tags and vault vocab. (Glossary lands in Plan B — until then, this step is a no-op.)

---

## Phase 2 — Shape

Draft before calling the CLI:

- **Question** — becomes the thread title; phrased as a question.
- **Initial angle** — body prose; what you'll explore first.
- **Known sources** — articles, videos, prior threads/learnings to seed the body.
- **Diataxis** — default `explanation`; switch to `reference` if the work is clearly cataloging known options.

**Gate:** confirm question + diataxis with the user before creating.

---

## Phase 3 — Create

**A. From inbox (promotion):**

```bash
anvil promote <inbox-id> --as thread
# output: "thread <new-thread-id>"
```

**B. Greenfield:**

```bash
anvil create thread --title "<question>" --json
# capture id + path from JSON
```

---

## Phase 4 — Bind active session

```bash
anvil thread activate <thread-id>
```

This writes `~/.anvil/state/active-thread`. Subsequent session captures will auto-stamp `session.related: [[thread.<id>]]` once the orchestrator session emitter consumes that state (separate work).

---

## Phase 5 — Author body

Direct-edit the file at the path returned by phase 3:

- The question (h1 or near-top).
- Working hypothesis or angle of attack.
- Known sources (article URLs, video links, prior threads/learnings).
- Open subquestions.

Edit body only — do not hand-author frontmatter.

---

## Phase 6 — Validate

```bash
anvil validate
```

Fix any schema errors reported. Re-run until clean.

---

## Hand-off

Three terminal paths. Name them in the closing message:

- **`anvil:distilling-learning`** — knowledge crystallized.
- **`anvil:writing-issue`** — project work surfaced; thread can stay open as parallel context.
- **abandon** — `anvil set thread <id> status abandoned`.

Closing/pausing the thread itself is a thin `anvil set thread <id> status closed|paused`. No separate skill.
