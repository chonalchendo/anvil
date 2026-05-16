---
name: handing-off-session
description: "Use at end of a working session to write a load-ready handoff into the current session file. Triggers: 'session handoff', 'wrap up the session', 'hand off to a new session', 'write the handoff'."
---

# Handing-off Session

Your job is to write one tight Markdown block into the current session file that `anvil:resuming-session` will load in the next terminal. The receiving agent has zero memory of this session but has the same shell, repo, and vault. Anything they can derive from `git`, `anvil list`, or `anvil show` does **not** belong in the handoff — name the query, don't paste the output.

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

## Brevity budget

The handoff body must aim for **≤1.2 KB** (≤1 KB pointer body + ≤200 B token-reflection block). A typical dogfood-loop handoff is 600–900 B before the reflection; anything past the cap needs every paragraph to justify itself against the cuts below. `anvil:resuming-session` loads this file verbatim every session — bloat compounds across the entire dogfood loop.

Section-by-section cuts to apply *before* writing, not after:

- **Just landed:** PR number + one phrase of impact. Never implementation detail; the new agent runs `gh pr view <n>` or `git log -p` if they need it.
- **Next action:** the *query*, never the list of candidate IDs the query returns. `anvil list issue --project <p> --ready` beats five enumerated IDs.
- **Open threads:** one line each, pointing to an artifact id (inbox slug, PR number, issue id). No paraphrase.
- **Don't redo:** approach + one-word reason. No reasoning chain.
- **Reminders:** if every candidate line restates AGENTS.md, omit the section entirely. AGENTS.md auto-loads. Keep only session-specific deltas (a transient env var, a one-off stash).
- **Token reflection:** 2–3 bullets, ≤200 B total. Top sinks (avoidable reads, redundant searches, oversized tool output) + one-phrase cut each. Not optional — a session with no token-side observation is itself a finding; write *"no avoidable sinks observed"* if true.

If a section would be empty after these cuts, omit the section header too. "Skip if empty" in the template is a hard rule, not a suggestion.

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

**Token reflection.** <2–3 bullets, ≤200 B. Top sinks this session → one-phrase cut. Required; satisfies the CLAUDE.md MUST. Write *"no avoidable sinks observed"* if none.>
```

## Phase 3 — Write into the session file and stop

Write the populated template into the body of the current session file at `~/anvil-vault/10-sessions/<session-id>.md` (the file the `session-start` hook created — its frontmatter already exists). Preserve the frontmatter; place the body under a `## Handoff` heading.

Do **not** emit a copy-pasteable block for the user. `anvil:resuming-session` reads the body directly from the session file in the next terminal — paste is dead weight.

Confirm the write in one line: `Handoff written to ~/anvil-vault/10-sessions/<session-id>.md.`

Do not offer to commit, push, or summarise further. The handoff is the deliverable.

## What does NOT go in the handoff

- Full conversation recap or chronological narrative.
- The list of every modified file (the new agent runs `git status`).
- Resolved issue bodies (the new agent runs `anvil show issue <id>`).
- Implementation detail of landed PRs (the new agent runs `gh pr view <n>` or `git log -p`).
- Enumerated candidate issue IDs from `anvil list --ready` — name the query, never the result set.
- Restating AGENTS.md / CLAUDE.md content (it auto-loads).
- "We learned that…" reflections — those belong in `anvil:distilling-learning`, not the handoff. **Exception:** token-cost observations (sinks + cuts) belong in the **Token reflection** section above — that satisfies the CLAUDE.md end-of-session MUST and has no other destination.
- TODOs the new agent should self-discover via `anvil list issue --ready`.

If the temptation to include any of the above appears, replace it with the one-line query that surfaces it.

## When the session has nothing handoff-worthy

If `git status` is clean, no new artifacts were created, and no decisions were reached: say so in one line — *"Nothing to hand off; new session starts from a clean tree."* — followed by the **Token reflection** bullets (still required). Do not invent next-actions to fill the template.
