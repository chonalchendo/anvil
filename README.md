# Anvil

A context management and workflow tool for agentic software development. You write the design first. Every milestone and issue traces to it. The agent implements each issue in isolation, gets an independent review and brings the PR to green. You approve each merge on evidence.

> **Status:** alpha, unreleased. Anvil is built with anvil, and its author uses it daily. Expect breaking changes to the vault schema.

## Why

AI-driven projects get messy as they grow. The agent starts every session with no memory, so the intent of the project lives in chat history. Decisions get made twice. Code drifts from the design. Agents start to build before anyone decides what to build.

Anvil makes the design the first artifact and the source of all work:

- **You design before you build.** Product design, system design and component designs come before code.
- **All work traces to design.** Work flows down one spine: design → milestone → issue. No task is an orphan.
- **The agent gets the right context at the right time.** `anvil hydrate <issue>` assembles every artifact that governs one issue, at the start of that issue.
- **Knowledge is recorded once and reused.** Decisions, research threads and learnings persist in one vault and cross project boundaries.
- **Gates are code, not prose.** Each issue carries a runnable verification. The worker cannot change it after the claim. The merge refuses without a pass at the PR head and a clean review.
- **The agent operates anvil.** You talk; skills load on intent and run the CLI. You remember no commands.
- **You read what the agent reads.** `anvil ui` serves the vault as a local, read-only web view.

Anvil is for solo developers who build real systems with agents over months and want to own the design. It is not for prototypes or one-off scripts, where design-first costs more than it returns.

## How it works

Anvil has four parts. The agent harness (Claude Code first) runs every agent; anvil never starts an agent process.

| Part | What it is |
|---|---|
| **Vault** | A git-versioned directory of typed, linked markdown files (default `~/anvil-vault`). It holds every project. It is the only source of truth. A SQLite index is a disposable cache of it. |
| **CLI** | One Go binary, `anvil`. It is the only way to change the vault. Every write validates against a JSON Schema and lands whole. |
| **Methodology bundle** | Skills, pinned-model subagents and a session guide. They tell the agent when to call the CLI and what to do with the result. |
| **Human view** | `anvil ui`, a read-only web view of the vault on localhost. |

Work runs in two loops:

1. **Outer loop — the human works at the milestone.** You choose a milestone, approve it and accept its result. A milestone holds an end goal, a runnable finish line and the design change. It holds no step list.
2. **Inner loop — anvil works at the issue.** Anvil derives the issues a few at a time, against the current code. For each issue, a worker agent implements it in its own git worktree and opens a PR. A second agent reviews it. Anvil fixes findings until verification, review and CI all pass at the PR head. Then you approve the merge.

Each loop has a limit. A stuck issue or an oversized milestone returns to you: amend, cut scope or abandon.

Four gates read vault evidence:

- **Create gate** — an issue's verification must fail before the fix, so it can tell fixed from broken.
- **Milestone gate** — no issue is claimed before its milestone is approved.
- **Land gate** — `--land-pr` re-runs the verification on a fresh checkout of the PR head and merges only on its pass, a clean review round and green CI.
- **Acceptance gate** — a milestone is done only when its finish line passes on the default branch and you accept the result.

## Install

Anvil is unreleased, so build from source. You need [Go](https://go.dev/dl/), [`just`](https://github.com/casey/just), [`gh`](https://cli.github.com/) and [Claude Code](https://claude.com/claude-code).

```bash
git clone https://github.com/chonalchendo/anvil.git
cd anvil
just install                     # version-stamped binary into $(go env GOPATH)/bin
anvil --version

anvil init --install-claude      # scaffold ~/anvil-vault; install skills, agents and hooks into ~/.claude
```

Restart Claude Code. The available-skills list now shows anvil's skills (`writing-issue`, `completing-issue`, …).

Use `just install`, not `go install ./cmd/anvil`. The recipe stamps the version and catches a stale binary that shadows your `$PATH`. After a rebuild, run `anvil install skills` to refresh the installed bundle.

To install the parts one at a time, or to remove them:

```bash
anvil install skills    # skills → ~/.claude/skills
anvil install agents    # subagents → ~/.claude/agents
anvil install hooks     # SessionStart, PreCompact and SessionEnd hooks
anvil install skills --uninstall
```

The hooks load the session guide at session start, turn each compaction into a handoff the next turn reloads, and commit and push the vault at session end.

## Start a project

In the root of your repo:

```bash
anvil project adopt <slug>
anvil where                      # confirm the vault and project
```

Then talk to the agent. Each request below fires a skill:

| You say | Skill |
|---|---|
| "Write the product design for this project." | `writing-product-design` |
| "Write the system design." | `writing-system-design` |
| "Design the internals of the X component." | `writing-component-design` |
| "What's the next milestone?" | `writing-milestone` |
| "Run the milestone." | `running-milestone` |
| "Work issue 0042." | `completing-issue` |
| "Open an issue for X." | `writing-issue` |
| "Let's research X." / "Open a thread on X." | `opening-thread` |
| "Idea: …" / "Park this." | `capturing-inbox` |
| "Record this learning." | `distilling-learning` |
| "Wrap up the session." / "Pick up where I left off." | `handing-off-session` / `resuming-session` |

Open the human view to read what the agent wrote:

```bash
anvil ui                         # http://127.0.0.1:7780
```

## The bundle

**Skills** load on intent. You do not call them by name.

| Group | Skills |
|---|---|
| Design | `writing-product-design`, `writing-system-design`, `writing-component-design`, `writing-convention` |
| Plan and run | `writing-milestone`, `writing-issue`, `running-milestone`, `completing-issue` |
| Review | `reviewing-pr`, `responding-to-pr-review` |
| Knowledge | `opening-thread`, `capturing-inbox`, `distilling-learning`, `refreshing-learnings` |
| Session | `handing-off-session`, `resuming-session` |

**Subagents** pin their own model, so cheap work runs on a cheaper model while the main thread plans:

| Agent | Job |
|---|---|
| `anvil-issue-worker` | Completes one issue to PR-opened. |
| `anvil-pr-reviewer` | Reviews one PR and returns structured findings. |
| `anvil-pr-responder` | Drives one PR's findings to fixes-pushed. |
| `anvil-issue-author` | Writes a batch of issues against one milestone. |
| `anvil-learnings-researcher` | Surfaces prior learnings before new work. |
| `anvil-researcher` | Runs one research topic and returns cited findings. |

## The vault

Artifact types: `product-design`, `system-design`, `component-design`, `convention`, `milestone`, `issue`, `decision`, `thread`, `learning`, `inbox`, `session`, `sweep`.

The files are plain markdown with YAML frontmatter, linked by `[[wikilinks]]`. Any editor can read them, and the vault stays readable if anvil disappears. Schemas live in [`schemas/`](schemas/); field reference in [`docs/vault-schemas.md`](docs/vault-schemas.md).

## The CLI

The agent runs the CLI. These verbs are also useful by hand:

| Verb | Use |
|---|---|
| `anvil where` | Print the current vault and project. |
| `anvil list <type>` / `anvil show <type> <id>` | Read artifacts. `list issue --ready` gives the work queue. |
| `anvil hydrate <issue>` | Assemble every artifact that governs one issue. |
| `anvil milestone status <id>` | Issue counts, per-issue cost and the finish line. |
| `anvil verify <issue>` | Run an issue's verification and record the verdict. |
| `anvil cost <issue>` | Review rounds, diff size and tokens for one issue. |
| `anvil doctor` | Flag stale state: merged-PR issues, dead claims, orphan worktrees, design drift. |
| `anvil validate` | Check the vault against the schemas. |
| `anvil ui` | Serve the human view. |

Run `anvil --help` for the full verb list. Most read verbs take `--json`.

## Other harnesses

The skills follow the open [SKILL.md](https://agentskills.io) standard, so they port to other agent CLIs. `--target` picks the harness:

| Harness | Skills | Agents |
|---|---|---|
| Claude Code (default) | `~/.claude/skills` | `~/.claude/agents` |
| Codex | `--target codex` | `--target codex` (TOML) |
| pi | `--target pi` | `--target pi` |
| Ante | `--target ante` | `--target ante` |
| OpenCode | reads `~/.claude/skills` | `--target opencode` |

Hooks are Claude Code only.

**Claude Code cloud sessions.** Attach your project and your vault repo to the session. Add `anvil install cloud` as a `SessionStart` hook in the project's committed `.claude/settings.json`. It links the vault, installs the bundle and builds the index. On a local machine the same hook does nothing.

## Contributing

Anvil develops itself. Its own product and system designs live in the vault:

```bash
anvil show product-design anvil --body
anvil show system-design anvil --body
```

[`AGENTS.md`](AGENTS.md) holds the rules for agents that work on this repo (`CLAUDE.md` links to it). [`docs/`](docs/) holds the Go, test, git and skill-authoring conventions.

## License

[MIT](LICENSE)
