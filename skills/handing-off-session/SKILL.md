---
name: handing-off-session
description: Use at end of a working session to produce a copy-pasteable prompt for the next session. Triggers include "session handoff", "wrap up the session", "give me a prompt for the next session", "hand off to a new session". Captures state and next action without re-narrating what the new agent can derive from `git log` / `anvil list --ready` / `anvil show`.
---

# Handing-off Session

Your job is to produce one tight Markdown block the user pastes into a new session. The receiving agent has zero memory of this session but has the same shell, repo, and vault. Anything they can derive from `git`, `anvil list`, or `anvil show` does **not** belong in the handoff — name the query, don't paste the output.

## Iron Law

**The handoff is a pointer document, not a recap.** If you find yourself summarising what was done, stop. Replace the summary with the command that surfaces it (`git log master..HEAD --oneline`, `anvil list issue --status resolved --since today`, etc.) and a one-line *why this matters for the next move*.

## Phase 1 — Gather state via queries (do not narrate)

Run these and read the output. Do not paste raw output into the handoff; use it to populate the template precisely.

```bash
git rev-parse --abbrev-ref HEAD                  # branch
git rev-parse --show-toplevel                    # worktree path
git status --short                               # uncommitted state
git log --oneline -5                             # recent commits
anvil list issue --project <p> --status open --ready --json   # what's pickable next
```

If the session resolved issues, also run `anvil list issue --status resolved` filtered to today (or grep recent transitions) — but only to confirm IDs you'll cite, not to recap.

## Phase 2 — Shape the prompt

Use exactly this template. Omit a section if it has nothing real to say; do **not** pad.

```markdown
Working in <repo path>. <One-sentence framing: what kind of work, which project.>

**State.** Branch `<name>` @ `<worktree path if not main checkout>`. <Clean | N files modified — name the load-bearing ones, not all of them.> Suite: <green / N failing / not run>.

**Just landed.** <0–3 bullets, each: artifact id or file → why it matters for the next move. Skip if nothing landed.>

**Next action.** <Verb-first single sentence. What the receiving agent does first.>

**Open threads.** <Only items NOT obvious from `anvil list --ready` or `git log`. Each one line. Skip section if empty.>

**Don't redo.** <Approaches considered and rejected this session, with one-line reason. Skip if nothing.>

**Reminders.** <Session-specific rules the receiving agent might not infer from AGENTS.md alone. Skip if nothing.>
```

## Phase 3 — Emit and stop

Print the populated template inside a fenced code block so the user can copy it in one click. Add nothing after the block except a single line: *"Paste into the next session."*

Do not offer to commit, push, or summarise further. The handoff is the deliverable.

## What does NOT go in the handoff

- Full conversation recap or chronological narrative.
- The list of every modified file (the new agent runs `git status`).
- Resolved issue bodies (the new agent runs `anvil show issue <id>`).
- Restating AGENTS.md / CLAUDE.md content (it auto-loads).
- "We learned that…" reflections — those belong in `anvil:distilling-learning`, not the handoff.
- TODOs the new agent should self-discover via `anvil list issue --ready`.

If the temptation to include any of the above appears, replace it with the one-line query that surfaces it.

## When the session has nothing handoff-worthy

If `git status` is clean, no new artifacts were created, and no decisions were reached: say so in one line — *"Nothing to hand off; new session starts from a clean tree."* Do not invent next-actions to fill the template.
