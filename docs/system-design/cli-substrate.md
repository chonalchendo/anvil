---
title: "Anvil system design — CLI substrate"
tags: [domain/dev-tools, type/system-design-shard]
---

## The `anvil` CLI (deterministic substrate)

The CLI is the **deterministic boundary** under the skills. Skills handle judgment; the CLI handles mechanics (paths, frontmatter generation, ID allocation, cross-references). This means: skills shrink, refactoring is safe, vault layout is invisible until needed.

Cold-start frequency is the load-bearing constraint — skills call the CLI dozens of times per session. Go's ~5–15ms cold start is effectively instant; Python's 80–200ms would disqualify it (10–20 seconds per session in pure overhead). Rust was considered (~3–10ms) but rejected for iteration friction. Go covers both the orchestrator and the CLI for this reason.

**Design rules:** boring, no interactive prompts, JSON output behind `--json`, stdout for content, stderr for diagnostics, meaningful exit codes, files stay editable by hand.

**Final v0.1 verb set** (uniform create/show/list/link/set over typed objects):

```
anvil where
anvil promote    <id> [flags]                # promote an inbox entry to a typed artifact
anvil create     <type> [flags]              # type ∈ {inbox, issue, plan, milestone, decision, learning, sweep, thread, session}
anvil show       <type> <id>
anvil list       <type> [--filters]
anvil link       <type> <id> --to <type> <id>
anvil set        <type> <id> <field> <value>
anvil tags       add | list | define
anvil project    list | switch | adopt | current
```

`anvil session log` was cut as redundant — session transcripts are written by the agent CLIs themselves; the active plan file is the canonical handoff.

**Reads split by shape.** Known-path content uses `Read`/`grep` directly — nothing to validate, and a wrapper just adds latency and a failure surface. Structured queries across typed frontmatter use `list <type> --filters`, where the SQLite index does joins `grep` can't. No `anvil read`.

**Edits split the same way.** `set` is for typed fields (validated like `create`); body prose stays raw markdown, edited in place.

`tags list` walks the vault and aggregates `tags` frontmatter into a deduped (tag, count) list. Used by artifact-creating skills to discover existing taxonomy before proposing new tags — minimizing tag drift.

**Project identity resolution** (three-step fallback): explicit `anvil project adopt <slug>` binding (recorded in `~/.anvil/projects/<slug>/.binding`) → git remote URL → refuse with clear error. The adopted binding takes precedence so an explicit user override always wins over the inferred one. No magic cwd-basename fallback.

**Indexing strategy:** SQLite-backed structured index of frontmatter is the next step when scale demands it; embedded vector DB is unlikely to ever be necessary for this workload — structured queries handle 95% of what naive intuition would reach for vectors for.

The v0.0.0-dev scaffold has none of this wired (cobra+fang lands when the first verb is implemented); this section documents the planned surface, not what runs today.

The session-emission path is the orchestrator-side of the thread→session→learning loop: a Claude Code `SessionStart` hook (installed via `anvil install hooks`) invokes the hidden `anvil install fire-session-start` wrapper, which writes a session artifact under `10-sessions/`, stamping `related: [[thread.<active>]]` if a thread is active. `distilling-learning` then walks that link to attach learnings back to the thread. See `docs/superpowers/specs/2026-05-02-session-emitter-design.md` for the full design.

**Skills auto-refresh.** `anvil install skills` materialises the binary's embedded skill bundle to `~/.anvil/skills/` and writes a content hash to `.anvil-skills-hash` alongside it. On every subsequent `anvil` invocation, the root command compares that on-disk hash against the hash computed from the embedded FS; on mismatch (typically after `go install ./cmd/anvil` rebuilt the binary with edited SKILL.md content), it rewrites the bundle in place and prints a one-line stderr notice. Install subcommands skip the check to avoid redundancy.
