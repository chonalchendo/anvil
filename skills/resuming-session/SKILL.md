---
name: resuming-session
description: Use at the start of a fresh terminal/session when the user wants to pick up prior work. Triggers include "resume session", "resuming session", "load last handoff", "load the handoff", "pick up where I left off", "what was I doing", "continue from last session". Not for ending a session — that is anvil:handing-off-session.
---

# Resuming Session

Your job is to locate the most-recent non-empty session handoff under `~/anvil-vault/10-sessions/`, load it into the current context, and surface the session id and path so the user can audit the source.

The companion **REQUIRED SIBLING:** `anvil:handing-off-session` writes the handoff at end-of-session; this skill reads it at the start of the next.

## Iron Law

**Surface the source before acting on the content.** Always print the session id and path of the handoff you loaded before doing anything else. The user must be able to audit what context just entered the conversation.

## Phase 1 — Locate the most-recent non-empty handoff

Walk session files newest-first by mtime (UUID filenames carry no order). Skip files that are frontmatter-only — `session-start` creates a stub before any handoff is written. Stop at the first file with content after the second `---` delimiter.

```bash
walked=0
chosen=""
# \ls bypasses shell aliases (eza, exa, etc.) that mangle plain `ls` output.
for f in $(\ls -t ~/anvil-vault/10-sessions/*.md 2>/dev/null); do
  if awk 'BEGIN{c=0} /^---$/{c++;next} c>=2 && NF{e=1;exit} END{exit !e}' "$f"; then
    chosen="$f"
    break
  fi
  walked=$((walked + 1))
done
echo "chosen=$chosen walked=$walked"
```

If no non-empty handoff exists anywhere under `10-sessions/`, stop. Tell the user verbatim: *"No prior handoff found under ~/anvil-vault/10-sessions/. Start fresh."* Do not invent context.

Remember `walked` for Phase 3.

## Phase 2 — Load the handoff body

Extract the session id from the filename (`<id>.md`) and read the body via the read-side verb:

```bash
anvil show session <id> --body
```

Read the output in full. The body is the user's instructions for this session — treat it as such.

## Phase 3 — Surface the source

Print exactly:

```
Loaded handoff: session <id>
Path: ~/anvil-vault/10-sessions/<id>.md
```

If `walked` ≥ 1, add: `Walked past <walked> empty session(s) to reach this one.`

Cross-check git state. Run `git -C <repo from handoff> rev-parse --abbrev-ref HEAD`. If the current branch differs from what the handoff describes, surface a single line: `Handoff describes branch <X>; current branch is <Y> — reconcile before acting.` Do not silently overwrite — let the user decide whether to switch branches, cut a fresh worktree, or proceed regardless.

After surfacing, hand control back to the user (or, if the handoff names an unambiguous next action and the user already invoked you with intent to continue, proceed with that next action).

## What NOT to do

- Do not auto-load on session start. This skill fires on explicit user invocation only — auto-loading would pollute sessions opened for unrelated work.
- Do not merge multiple handoffs. Only the most-recent non-empty one. Stitching across sessions belongs in `anvil:distilling-learning`.
- Do not summarise or paraphrase the handoff before acting on it. The handoff is the user's prompt, not raw material.
- Do not delete or mutate empty session files. Retention belongs elsewhere (`retention_until` frontmatter + a future sweep verb).
- Do not invent a next action when the handoff says *"Nothing to hand off; new session starts from a clean tree."* Surface that, ask the user what to work on.
