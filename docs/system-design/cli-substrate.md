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
anvil link       <type> <id> <type> <id> [--relation depends_on|blocks]   # write edge (default related[]); --from/--to/--unresolved query
anvil set        <type> <id> <field> <value>
anvil append     <type> <id> --body-file <f> # append a validated body section, bumping updated
anvil tags       add | list | define
anvil index      <id> | --tags <facet/value,...>   # related artifacts by shared facets + links
anvil project    list | switch | adopt | current
```

`anvil session log` was cut as redundant — session transcripts are written by the agent CLIs themselves; the active plan file is the canonical handoff.

**Reads split by shape.** Known-path content uses `Read`/`grep` directly — nothing to validate, and a wrapper just adds latency and a failure surface. Structured queries across typed frontmatter use `list <type> --filters`, where the SQLite index does joins `grep` can't. No `anvil read`.

**Edits split the same way.** `set` is for typed fields (validated like `create`); `append` grows the body through the same validation; other body edits stay raw markdown in place.

`tags list` walks the vault and aggregates `tags` frontmatter into a deduped (tag, count) list. Used by artifact-creating skills to discover existing taxonomy before proposing new tags — minimizing tag drift.

`index` answers "what else is relevant to *this*?" — seed it with an artifact id or a tag set and it ranks artifacts by shared facets (plus a bonus for a direct link to an id seed), each row carrying the matched tags/links as evidence. Backed by the `tags` table in `.anvil/vault.db`. Used before create/complete/distill to pull related prior context into view.

**Project identity resolution** (three-step fallback): explicit `anvil project adopt <slug>` binding (recorded in `~/.anvil/projects/<slug>/.binding`) → git remote URL → refuse with clear error. The adopted binding takes precedence so an explicit user override always wins over the inferred one. No magic cwd-basename fallback.

**Indexing strategy:** SQLite-backed structured index of frontmatter is the next step when scale demands it; embedded vector DB is unlikely to ever be necessary for this workload — structured queries handle 95% of what naive intuition would reach for vectors for.

The v0.0.0-dev scaffold has none of this wired (cobra+fang lands when the first verb is implemented); this section documents the planned surface, not what runs today.

The session-emission path is the orchestrator-side of the thread→session→learning loop: a Claude Code `SessionStart` hook (installed via `anvil install hooks`) invokes the hidden `anvil install fire-session-start` wrapper, which writes a session artifact under `10-sessions/`, stamping `related: [[thread.<active>]]` if a thread is active. `distilling-learning` then walks that link to attach learnings back to the thread. See `docs/superpowers/specs/2026-05-02-session-emitter-design.md` for the full design.

**Skills install.** `anvil install skills` materialises the binary's embedded skill bundle to `<config-dir>/.anvil-skills-src/` (the resolved `--target` config dir — `~/.claude`, `~/.codex`, or `~/.pi/agent`, honoring `CLAUDE_CONFIG_DIR`/`CODEX_HOME`/`PI_CODING_AGENT_DIR`; `ANVIL_SKILLS_DIR` overrides outright) and writes a content hash to `.anvil-skills-hash` alongside it. Install, uninstall, and `anvil init --install-claude` all resolve that dir through the same helper, so exactly one materialise dir exists per target. The bundle changes only under this explicit verb — no other verb reads or rewrites it, so two differently-embedded binaries (e.g. a worktree build and the shared install) can never ping-pong the installed bundle between their contents. `anvil install skills` compares the on-disk hash against the invoking binary's embedded FS and re-materialises on mismatch (typically after `go install ./cmd/anvil` rebuilt the binary with edited SKILL.md content); re-running `--target codex` or `--target pi` refreshes a stale bundle the same way.
