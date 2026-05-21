# Anvil

A craft-first methodology for AI-assisted development, packaged as auto-loading [SKILL.md](https://agentskills.io) files with a thin Go orchestrator. Stubborn on vision, flexible on implementation — built to make you a stronger engineer, not just ship faster.

> **Status:** Alpha. v0.1 in active development — not yet released.

## What it is

Anvil treats a project's design as the load-bearing artifact and works backwards from it: **product-design → milestones → plans → sweeps → issues → inbox**. Every task traces up to a milestone; every milestone traces up to the product's purpose.

The methodology lives in **skills** — auto-firing markdown files (following Anthropic's open standard) the agent loads from conversational triggers, not commands you type. The **orchestrator** is a small Go CLI for the parts that genuinely need a process: vault state, scaffolding, and dispatching work to agent CLIs (Claude Code first, Codex next).

Two stores:

- **Knowledge vault** at `~/anvil-vault/` — issues, plans, milestones, learnings, decisions, skills, MOCs. Git-versioned, browsable in Obsidian, indexed by a local SQLite database.
- **Machine-local state** at `~/.anvil/` — project bindings, the bundled skills, and run state.

Your project repos stay clean: no Anvil-specific files by default.

## Why it exists

Anvil is the successor to [mantle](https://github.com/chonalchendo/mantle). Mantle worked but accumulated complexity — 30+ slash commands, heavy compiled context, Claude-only lock-in. Anvil keeps the load-bearing parts (lifecycle skills, vault, telemetry) and trades the orchestrator-heavy approach for a skill-based one that travels with you across projects rather than being scaffolded into each repo.

## Install

Install the binary with Go:

    go install github.com/chonalchendo/anvil/cmd/anvil@latest

This drops `anvil` into `$(go env GOPATH)/bin` — make sure that directory is on your `$PATH`. Verify with `anvil --version`.

Then scaffold a vault and wire Anvil into Claude Code:

    anvil init              # scaffold a vault (defaults to ~/anvil-vault)
    anvil install skills    # make the bundled skills available to Claude Code
    anvil install hooks     # bind each session to the active thread

Pass `--uninstall` to either `install` command to remove it.

Skills are discovered at session start, so **restart Claude Code** after `anvil install skills`. In a fresh session the available-skills list should include Anvil's skills (`writing-issue`, `completing-issue`, `capturing-inbox`, …), appearing bare without an `anvil:` prefix.

## Design & conventions

- [`docs/product-design.md`](docs/product-design.md) — vision, users, scope, milestones.
- [`docs/system-design.md`](docs/system-design.md) — architecture, vault structure, schemas (shards in [`docs/system-design/`](docs/system-design/)).
- [`AGENTS.md`](AGENTS.md) — how to write code for Anvil (`CLAUDE.md` is a symlink for Claude Code).

## Roadmap

- **v0.1** — minimal usable Anvil: vault scaffolding, core lifecycle skills, and `anvil build` with the Claude Code adapter (sequential execution). JSON Schema validation in CI.
- **v0.2** — Codex adapter; concurrent wave execution via git worktrees; brownfield onboarding.
- **v0.3** — educational gating workflow; workspaces for cross-repo coordination.
- **v0.4+** — iterate from real signal.

## License

[MIT](LICENSE)
