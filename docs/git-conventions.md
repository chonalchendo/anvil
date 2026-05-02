# Git Conventions

Commit message format and what must never enter the repository.

## Commit Messages

Conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `chore:`, `refactor:`, `release:`.

## What Never to Commit

- Credentials of any kind (real `auth.json`, API keys, tokens).
- Real API keys in tests — use sentinel values like `sk-test-fake`.
- Personal vault content (your `~/anvil-vault/`).
- Anything from `~/.anvil/` or `~/.claude/projects/`.
- `.env` files. Use `.env.example` as a template.
- Output artifacts (`bin/`, `dist/`, `*.test`, `coverage.out`).
