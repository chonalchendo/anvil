# Anvil

A craft-first methodology for AI-assisted development, packaged as auto-loading [SKILL.md](https://agentskills.io) files with a thin Python orchestrator.

> **Status:** Alpha. In active development. Not yet usable.

## What it is

Anvil treats the project's design as the load-bearing artifact and works backwards from it: **product-design → milestones → plans → sweeps → issues → inbox**. Every commit traces back to a milestone, every milestone traces back to the product's purpose. Stubborn on vision, flexible on implementation.

The methodology lives in skills the agent navigates (auto-firing markdown files following Anthropic's open standard); the orchestrator is a small Python CLI that handles project state, vault scaffolding, and subprocess execution against agent CLIs (Claude Code first, Codex next).

Two storage tiers:

- **Operational state** at `~/.anvil/` — issue files, briefings, build cache, telemetry. Per-project, machine-local.
- **Knowledge vault** at `~/anvil-vault/` — learnings, decisions, milestones, skills, MOCs. Browsed in Obsidian. Git-versioned.

Project repos contain no Anvil-specific files by default. Work-friendly, no ceremony in repos that aren't yours to modify.

## Why it exists

Anvil is the successor to [mantle](https://github.com/chonalchendo/mantle). Mantle worked but accumulated complexity — 30+ slash commands, heavy compiled context, Claude-only lock-in, weak build step. Anvil keeps the load-bearing parts (lifecycle hooks, vault, telemetry) and trades the orchestrator-heavy approach for a skill-based one. The methodology travels with the user across projects rather than being scaffolded into each repo.

## Design

The full design lives in [`docs/design.md`](docs/design.md). It covers architecture, vault structure, frontmatter schemas, the build command, skill authoring conventions, and implementation sequence. Read it before contributing.

The agent conventions (how to write code for Anvil) live in [`AGENTS.md`](AGENTS.md), with `CLAUDE.md` as a symlink for Claude Code compatibility.

## Roadmap

- **v0.1** — `anvil init`, `anvil status`, `anvil cost`, `anvil compile`, `anvil build` with Claude Code adapter (sequential execution). Core methodology skills (~11). Vault scaffolding. JSON Schema validation in CI.
- **v0.2** — Codex adapter. Concurrent wave execution with git worktrees. Brownfield adoption (`anvil adopt`). Knowledge-skill lifecycle (`researching-domain`, `synthesizing-knowledge-skill`).
- **v0.3** — Educational gating workflow. Workspace concept for cross-repo coordination.
- **v0.4+** — Refinements based on real use.

## Setup

Install the binary with Go:

    go install github.com/chonalchendo/anvil/cmd/anvil@latest

This drops `anvil` into `$(go env GOPATH)/bin`. Make sure that directory is on your `$PATH` (e.g. `export PATH="$(go env GOPATH)/bin:$PATH"` in your shell rc). Verify with `anvil --version` — installs from a tagged module report the module version; local `go install ./cmd/anvil` from a checkout reports `dev-<sha7>`.

Then wire Anvil into Claude Code:

    anvil install skills    # symlinks ~/.claude/skills/anvil -> ~/.anvil/skills (use --copy on filesystems without symlinks)
    anvil install hooks     # SessionStart hook binds each session to the active thread

To remove either, pass `--uninstall` (e.g. `anvil install skills --uninstall`).

### Verify skills loaded

Skills are discovered by Claude Code at session start, so after `anvil install skills` you need to **restart Claude Code** — exit your current session and launch a fresh one. Inside that new session, the available-skills listing should include anvil's skills (`writing-issue`, `writing-plan`, `capturing-inbox`, …).

They appear **bare**, without an `anvil:` prefix. That's intentional for v0.1; see the decision record:
`anvil show decision skills-namespace-prefix.0001-defer-anvil-namespace-prefix-for-skills-until-v0-1-distribut --body`.

## License

[MIT](LICENSE)
