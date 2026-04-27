# Anvil — Agent Conventions

Anvil is a methodology for AI-assisted development packaged as auto-loading SKILL.md files with a thin Python orchestrator. The architectural design lives in `docs/design.md`. Read it before making structural decisions.

This file captures *how to write code* for Anvil. The design doc captures *what Anvil is*.

These rules apply universally. Task-specific guidance lives in `docs/`.

## Behavioral Guardrails

### Think Before Coding

Don't assume. Don't hide confusion. Surface tradeoffs.

Before implementing:

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

**YOU MUST stop and ask the user, not guess, when:**

- Adding a new dependency. Even a small one.
- Choosing between two genuinely different architectural approaches.
- Refactoring code outside the scope of the current task.
- Adding a new artifact type, skill, schema, or top-level directory.
- The design doc is silent or ambiguous on a structural question.
- Implementation is taking longer than expected and you're considering shortcuts.

Don't stop and ask for: trivial naming choices, where to put a clearly-bounded helper, formatting decisions covered by ruff config.

### Surgical Changes

Touch only what you must. Clean up only your own mess.

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it — don't delete it.

When your changes create orphans:

- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

**The test: every changed line should trace directly to the user's request.**

### Goal-Driven Execution

Define success criteria. Loop until verified.

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:

    1. [Step] → verify: [check]
    2. [Step] → verify: [check]
    3. [Step] → verify: [check]

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## Code Design

### Core Principles

| Principle                  | Rule                                                                |
| -------------------------- | ------------------------------------------------------------------- |
| Deep Modules               | Interface small relative to functionality; resist "classitis"       |
| Information Hiding         | Each module encapsulates design decisions; no knowledge duplication |
| Pull Complexity Down       | Absorb complexity into implementation, not callers                  |
| Define Errors Out          | Redefine semantics so problematic conditions aren't errors          |
| General-Purpose Interfaces | Design for the general case; handle edge conditions internally      |
| Design It Twice            | Consider two radically different approaches before committing       |

### Red Flags

| Red Flag                | Signal                                              |
| ----------------------- | --------------------------------------------------- |
| Shallow Module          | Interface complexity ≈ implementation complexity    |
| Information Leakage     | Same design decision in multiple modules            |
| Repetition              | Near-identical code in multiple places              |
| Special-General Mixture | General mechanism contains use-case-specific code   |
| Vague Name              | Name too broad to convey specific meaning           |
| Hard to Describe        | Can't write a simple comment → revisit the design   |
| Nonobvious Code         | Behaviour requires significant effort to trace      |

### Common Rationalizations

| Rationalization                | Why It's Wrong                                            | What to Do Instead                                                |
| ------------------------------ | --------------------------------------------------------- | ----------------------------------------------------------------- |
| "More classes = better design" | More interfaces to learn, not simpler code                | Apply the complexity test — does splitting reduce cognitive load? |
| "Keep methods under N lines"   | Over-extraction creates pass-throughs and conjoined logic | Extract only when the piece has a meaningful name and concept     |
| "Add a config parameter"       | Pushes complexity to every caller                         | Compute sensible defaults inside the module                       |
| "Structure by execution order" | Causes information leakage across all steps               | Structure by information boundaries instead                       |
| "We might need this later"     | Speculative generality adds interface surface now         | "Somewhat general-purpose" — cover plausible uses only            |

## Hard Rules

- **No helper without a second use.** Don't extract until duplication exists.
- **No abstraction without need.** Premature abstraction creates surface that costs more than it saves.
- **No defensive code for unreachable states.** If a precondition is invariant, document it; don't check it at runtime.
- **No comments explaining *what*.** Comments explain *why* the code is shaped this way, never restate what the code does.
- **No `print()` for control flow output.** CLI output goes through Typer's echo helpers; logging goes through the `logging` module with `%s` formatting (not f-strings).
- **No new top-level dependencies without explicit user approval.**

If you write 200 lines and it could be 50, rewrite it. Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## Python Conventions

### Imports

- **Module-alias imports** for internal code: `import anvil.adapters.base as base`, then `base.AgentAdapter(...)`.
- Reserve `from x import Y` for single-symbol stdlib usage (e.g., `from pathlib import Path`).
- `TYPE_CHECKING` guard for annotation-only imports.

### Type Annotations

- Annotate all public functions (params + return).
- Use `X | Y` union syntax, not `Optional[X]`.
- No `from __future__ import annotations` (3.12+).
- Avoid `Any`.

### Error Handling

- Catch specific exceptions — never bare `except Exception:` unless re-raising.
- If catching and not re-raising, log at WARNING minimum.
- Use `raise X from Y` to preserve chains.

### Functions

- Max 5 parameters. Beyond that, group into a config object or dataclass.
- Keyword-only (`*`) for boolean params and optional params after the first two positional.

### Docstrings

- Google style. One-liner for trivial functions.
- Module docstring: one sentence describing purpose.

## Test Conventions

- Framework: pytest.
- Test directory mirrors `src/anvil/` structure (e.g. `tests/core/test_manifest.py` for `src/anvil/core/manifest.py`).
- Top-level `tests/test_package.py` and `tests/test_workflows.py` for project-wide and infrastructure tests.
- Use `tmp_path` for isolated file operations — **never touch real `~/.claude/` or real vaults**.
- Mock subprocess calls at the `asyncio.create_subprocess_exec` boundary; real-CLI tests live in `tests/integration/` gated behind `pytest -m integration`.

## What Never to Commit

- Credentials of any kind (real `auth.json`, API keys, tokens).
- Real API keys in tests — use sentinel values like `sk-test-fake`.
- Personal vault content (your `~/anvil-vault/`).
- Anything from `~/.anvil/` or `~/.claude/projects/`.
- `.env` files. Use `.env.example` as a template.
- Output artifacts (`dist/`, `build/`, `*.egg-info`).

## Commit Messages

Conventional commits: `feat:`, `fix:`, `docs:`, `test:`, `chore:`, `refactor:`, `release:`.

## Reference Documents

### Releasing — `@docs/releasing.md`

**Read when:** cutting a new version. Covers `uv version` bump, README/CHANGELOG updates, tag-and-push, and the `publish.yml` workflow.

---

These conventions are working if: fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.