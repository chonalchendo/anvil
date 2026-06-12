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

First run `anvil doctor --json`. If `findings` is non-empty, surface each on one line — `[<kind>] <id> — fix: <fix>` — so stale lifecycle state is visible before any work is picked up. Doctor is read-only and fails open offline (empty findings, skip silently); never auto-run the fix commands — each correction stays an explicit agent/human action.

`anvil session resume --json` applies the 10-min ambiguity window, walks past empty stubs, and returns in one call. When the user specifies a project (e.g. "resume session for anvil"), add `--project <p>` to scope candidates to that project only:

- **No match** → `{walked, no_handoff: true}` (only when `--project` matched no handoff) — stop. Tell the user: *"No handoff found for project `<p>`. Start fresh."* Do not load the empty body. `no_handoff` is the explicit signal; a populated single-candidate hit always carries a non-empty `session_id`, so never treat `{session_id: ""}` as a candidate.
- **Single candidate** → `{session_id, path, objective, body, walked}` — proceed to Phase 3 directly with the loaded body.
- **Multiple candidates** → `{walked, candidates: [...]}` with `body` empty — the verb surfaces the list for you. Disambiguate before loading:

```text
Multiple recent handoffs in the ambiguity window:
  1) <id-short>  <HH:MM>  <objective or title>
  2) <id-short>  <HH:MM>  <objective or title>
  ...
Which one?
```

Use `AskUserQuestion` (or equivalent) with one option per candidate. Do not auto-pick on the user's behalf.

If the command errors ("no prior handoff found"), stop. Tell the user verbatim: *"No prior handoff found under ~/anvil-vault/10-sessions/. Start fresh."* Do not invent context.

Remember `session_id`, `path`, and `walked` for Phase 3.

## Phase 2 — Load the handoff body (disambiguation path only)

When Phase 1 returned multiple candidates and the user has chosen one, load the chosen body:

```bash
anvil session show <session_id> --body
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

## Phase 4 — Load active contracts

Before handing control back, run:

```bash
anvil list contract --json
```

Surface the contract descriptions in one compact block (name + description only — no bodies). This gives the agent a cheap boundary map so contract `does not` constraints are visible before any work begins. If the vault has no contracts, skip silently.

After surfacing, hand control back to the user (or, if the handoff names an unambiguous next action and the user already invoked you with intent to continue, proceed with that next action).

## What NOT to do

- Do not auto-load on session start. This skill fires on explicit user invocation only — auto-loading would pollute sessions opened for unrelated work.
- Do not merge multiple handoffs. Only the most-recent non-empty one. Stitching across sessions belongs in `distilling-learning`.
- Do not summarise or paraphrase the handoff before acting on it. The handoff is the user's prompt, not raw material.
- Do not delete or mutate empty session files. Retention belongs elsewhere (`retention_until` frontmatter + a future sweep verb).
- Do not invent a next action when the handoff says *"Nothing to hand off; new session starts from a clean tree."* Surface that, ask the user what to work on.
