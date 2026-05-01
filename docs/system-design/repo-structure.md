---
title: "Anvil system design — repository structure"
tags: [domain/dev-tools, type/system-design-shard]
---

## Repository structure

Source repo:

```
anvil/                          # source repo
├── cmd/anvil/main.go
├── internal/
│   ├── cli/                    # cobra root + fang wrap
│   ├── core/                   # wave executor, manifest, skill registry
│   ├── adapters/               # AgentAdapter implementations
│   ├── telemetry/              # SQLite event store
│   ├── installer/              # skill + template install
│   └── templates/              # embedded text/template assets
├── skills/                     # auto-loaded SKILL.md
├── schemas/                    # JSON schemas (deferred)
├── docs/
├── go.mod
├── tool.go.mod                 # isolated tool deps (golangci-lint, goreleaser, govulncheck, gotestsum)
└── justfile
```

Operational state (per-machine, abbreviated; see `design.md` § Repository structure for the full layout including `build-runs/<run-id>/<task-id>/`):

```
~/.anvil/                       # operational state (per-machine)
├── projects/<project>/         # in-progress work, briefs (no issues — see vault 70-issues/)
├── state/                      # per-spawn CLAUDE_CONFIG_DIR / CODEX_HOME
└── telemetry.db                # SQLite
```

Knowledge vault (Obsidian) gets its own shard — see [Knowledge base](knowledge-base.md).

The three trees are separate by invariant. Vault content is never committed to the project source repo; the source repo is never written into the vault; operational state is local to the machine and never touches either.

`tool.go.mod` keeps developer-tool dependencies (linter, release tooling) isolated from the runtime module graph so `go install github.com/.../anvil/cmd/anvil@latest` stays clean.
