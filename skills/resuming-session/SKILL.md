---
name: resuming-session
description: Use at the start of a fresh terminal/session when the user wants to pick up prior work. Triggers include "resume session", "resuming session", "load last handoff", "load the handoff", "pick up where I left off", "what was I doing", "continue from last session". Not for ending a session — that is anvil:handing-off-session.
---

# Resuming Session

Your job is to locate the right non-empty session handoff under `~/anvil-vault/10-sessions/` (disambiguating with the user when ≥2 landed near-simultaneously), load it into the current context, and surface the session id and path so the user can audit the source.

The companion **REQUIRED SIBLING:** `anvil:handing-off-session` writes the handoff at end-of-session; this skill reads it at the start of the next.

## Iron Law

**Surface the source before acting on the content.** Always print the session id and path of the handoff you loaded before doing anything else. The user must be able to audit what context just entered the conversation.

## Phase 1 — Locate candidate handoff(s) in the recency window

Walk session files newest-first by mtime (UUID filenames carry no order). Skip files that are frontmatter-only — `session-start` creates a stub before any handoff is written. A non-empty handoff has content after the second `---` delimiter.

Collect every non-empty handoff whose mtime is within `600` seconds (10 min) of the newest non-empty handoff's mtime. Stop at the first gap > 600 s. This is the *ambiguity window* — handoffs landing this close together can collide silently on a pure newest-by-mtime pick (cross-repo and, more dangerously, intra-repo).

```bash
walked=0
window_sec=600
candidates=()
newest_mtime=""
# \ls bypasses shell aliases (eza, exa, etc.) that mangle plain `ls` output.
for f in $(\ls -t ~/anvil-vault/10-sessions/*.md 2>/dev/null); do
  if ! awk 'BEGIN{c=0} /^---$/{c++;next} c>=2 && NF{e=1;exit} END{exit !e}' "$f"; then
    walked=$((walked + 1))
    continue
  fi
  m=$(stat -f %m "$f" 2>/dev/null || stat -c %Y "$f")  # darwin || linux
  if [ -z "$newest_mtime" ]; then
    newest_mtime=$m
    candidates+=("$f")
  elif [ $((newest_mtime - m)) -le $window_sec ]; then
    candidates+=("$f")
  else
    break
  fi
done
echo "candidates=${#candidates[@]} walked=$walked"
```

If `candidates` is empty, no non-empty handoff exists anywhere under `10-sessions/`. Stop. Tell the user verbatim: *"No prior handoff found under ~/anvil-vault/10-sessions/. Start fresh."* Do not invent context.

If `candidates` has exactly one entry, that file is the chosen handoff — proceed to Phase 2 silently.

If `candidates` has two or more entries, **disambiguate before loading**. For each candidate, extract the session id from the filename and render a preview: the first non-blank, non-heading body line (`awk` matcher `c>=2 && NF && !/^#/`), truncated to ~80 chars. This pulls the substantive opener (typically `Working in <repo>. ...`) rather than the `## Handoff` heading. Format:

```text
Multiple recent handoffs in the ambiguity window:
  1) <id-short>  <HH:MM>  <first substantive body line>
  2) <id-short>  <HH:MM>  <first substantive body line>
  ...
Which one?
```

Use `AskUserQuestion` (or equivalent) with one option per candidate. The list is newest-first; if the user picks the top entry they confirm what newest-by-mtime would have picked, but they do so explicitly. Do not auto-pick on the user's behalf.

Remember `walked` for Phase 3.

## Phase 2 — Load the handoff body

Extract the session id from the filename (`<id>.md`) and read the body via the read-side verb:

```bash
anvil show session <id> --body
```

Read the output in full. The body is the user's instructions for this session — treat it as such.

## Phase 3 — Surface the source

Print exactly:

```text
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
