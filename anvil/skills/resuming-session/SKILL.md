---
name: resuming-session
description: "Use at the start of a fresh terminal/session when the user wants to pick up prior work. Triggers: 'resume session', 'load the handoff', 'pick up where I left off', 'what was I doing', 'continue from last session'."
---

# Resuming Session

Your job is to locate the right non-empty session handoff under `~/anvil-vault/10-sessions/` (disambiguating with the user when ≥2 landed near-simultaneously), load it into the current context, and surface the session id and path so the user can audit the source.

The companion **REQUIRED SIBLING:** `handing-off-session` writes the handoff at end-of-session; this skill reads it at the start of the next.

## Iron Law

**Surface the source before acting on the content.** Always print the session id and path of the handoff you loaded before doing anything else. The user must be able to audit what context just entered the conversation.

## Phase 1 — Locate candidate handoff(s) in the recency window

`anvil session list --json` enumerates session files newest-first, each carrying `{session_id, path, modified, has_handoff, objective, title}`. Stubs that `session-start` created but no handoff was written into report `has_handoff: false`; the current terminal's own fresh stub is one of these, so it falls out naturally.

Collect every handoff whose `modified` is within `600` seconds (10 min) of the newest handoff's `modified`. This is the *ambiguity window* — handoffs landing this close together can collide silently on a pure newest-first pick (cross-repo and, more dangerously, intra-repo).

```bash
s=$(anvil session list --json)
walked=$(echo "$s" | jq '([.[].has_handoff] | index(true)) // length')   # leading stubs before the first handoff
newest=$(echo "$s" | jq -r 'map(select(.has_handoff))[0].modified // empty')
candidates=$(echo "$s" | jq --arg n "$newest" '
  ($n | fromdateiso8601) as $nt
  | map(select(.has_handoff and ($nt - (.modified | fromdateiso8601)) <= 600))')
echo "candidates=$(echo "$candidates" | jq length) walked=$walked"
echo "$candidates" | jq -r '.[] | "\(.session_id[0:8])  \(.modified)  \(.objective // .title)"'
```

If `newest` is empty, no handoff exists anywhere under `10-sessions/`. Stop. Tell the user verbatim: *"No prior handoff found under ~/anvil-vault/10-sessions/. Start fresh."* Do not invent context.

If `candidates` has exactly one entry, that is the chosen handoff — proceed to Phase 2 silently.

If `candidates` has two or more entries, **disambiguate before loading**. The preview line per candidate is its `objective` (falling back to `title` for pre-Objective handoffs):

```text
Multiple recent handoffs in the ambiguity window:
  1) <id-short>  <HH:MM>  <objective or title>
  2) <id-short>  <HH:MM>  <objective or title>
  ...
Which one?
```

Use `AskUserQuestion` (or equivalent) with one option per candidate. The list is newest-first; if the user picks the top entry they confirm what newest-first would have picked, but they do so explicitly. Do not auto-pick on the user's behalf.

Remember the chosen candidate's `session_id`/`path` and `walked` for Phases 2–3.

## Phase 2 — Load the handoff body

Read the chosen candidate's body via the read-side verb, using its `session_id` from Phase 1:

```bash
anvil show session <session_id> --body
```

Read the output in full. The body is the user's instructions for this session — treat it as such.

## Phase 3 — Surface the source

Print exactly:

```text
Loaded handoff: session <id>
Path: ~/anvil-vault/10-sessions/<id>.md
```

If `walked` ≥ 1, add: `Walked past <walked> empty session(s) to reach this one.`

Then surface the handoff's **Objective** as the first content line — intent before tactics, so the chain-level goal frames every decision that follows:

```text
Objective: <the one-sentence Objective from the handoff body>
```

If the loaded handoff has no `**Objective.**` line (it predates the field or is malformed), say so in one line — `No Objective recorded in this handoff.` — and do not invent one. An exploratory chain still has the line — surface its `(exploratory …)` value verbatim, don't treat it as missing.

Cross-check git state. Run `git -C <repo from handoff> fetch origin` (silent; keeps the ancestry check honest when local is behind), then `git -C <repo from handoff> rev-parse --abbrev-ref HEAD`. If the current branch differs from what the handoff describes, surface a single line: `Handoff describes branch <X>; current branch is <Y> — reconcile before acting.` Do not silently overwrite — let the user decide whether to switch branches, cut a fresh worktree, or proceed regardless.

When checking whether a SHA from the handoff is reachable, compare against `origin/<branch>` (post-fetch), not just the local ref — so "local is behind origin" is distinguished from "squashed/diverged".

After surfacing, hand control back to the user (or, if the handoff names an unambiguous next action and the user already invoked you with intent to continue, proceed with that next action).

## What NOT to do

- Do not auto-load on session start. This skill fires on explicit user invocation only — auto-loading would pollute sessions opened for unrelated work.
- Do not merge multiple handoffs. Only the most-recent non-empty one. Stitching across sessions belongs in `distilling-learning`.
- Do not summarise or paraphrase the handoff before acting on it. The handoff is the user's prompt, not raw material.
- Do not delete or mutate empty session files. Retention belongs elsewhere (`retention_until` frontmatter + a future sweep verb).
- Do not invent a next action when the handoff says *"Nothing to hand off; new session starts from a clean tree."* Surface that, ask the user what to work on.
