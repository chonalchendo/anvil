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

- The user is dumping a thought without committing → `capturing-inbox`.
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
- **Topic** — the slug scoping the thread's ordinal (`<topic>.<NNNN>-<slug>`) and the join key to the decision the thread may close into. Reuse an existing topic where one fits (`anvil list thread`, `anvil list decision`); project-tied research uses the project slug as its topic.
- **Initial angle** — body prose; what you'll explore first.
- **Known sources** — articles, videos, prior threads/learnings to seed the body.
- **Diataxis** — default `explanation`; switch to `reference` if the work is clearly cataloging known options.

**Gate:** confirm question + topic + diataxis with the user before creating.

---

## Phase 3 — Create

**A. From inbox (promotion):**

```bash
anvil promote <inbox-id> --as thread --topic <topic>
# output: "thread <new-thread-id>"
```

**B. Greenfield:**

```bash
anvil create thread --title "<question>" --topic <topic> --json
# capture id + path from JSON; id is <topic>.<NNNN>-<slug>
```

`--topic` is required either way — the ordinal is allocated per topic.

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

---

## Closing verdict

A thread is a workspace, not an output. Before closing one, rule on where its conclusion goes — the answer must outlive the file. Pick exactly one:

- **Decision** — the thread settled a standing choice (a rule, a default, a rejected option). Mint it **under the thread's own topic** so the two sort together and the topic reads as one arc, link it, then close:

  ```bash
  anvil create decision --title "<the choice>" --topic <same-topic-as-the-thread> --json
  anvil link thread <thread-id> decision <decision-id>
  anvil transition thread <thread-id> closed
  ```

- **Learning** — the thread produced transferable knowledge rather than a choice. Fire `distilling-learning`, link the result, then close.

- **Nothing durable** — the thread resolved to a one-off factual answer with nothing to carry forward. Close (or `abandoned` when the question dissolved) plainly, with no artifact:

  ```bash
  anvil transition thread <thread-id> closed
  ```

`anvil transition thread <id> closed` warns when the thread carries no outbound decision or learning link. It never blocks — the third verdict is legitimate — but treat the warning as the prompt to confirm you *chose* it rather than skipped the ruling.

Project work surfaced mid-thread routes to `writing-issue` and is not a closing verdict: the thread stays open as parallel context.

Pausing is a plain `anvil transition thread <id> paused`. No separate closing skill.
