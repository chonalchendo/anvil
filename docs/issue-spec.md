# Anvil Issue Body Spec

Body-section contract for anvil issues. Authored by `writing-issue`; consumed by `completing-issue` and `skills/completing-issue/scripts/run-verification.sh`.

For frontmatter, see [vault-schemas.md](vault-schemas.md#issue). The issue's terminal predicate lives in the required `goal:` frontmatter field — one sentence, ≤120 chars, naming what "done" means. The body sections below carry the detail.

## Required body sections

- `## Problem` — one paragraph. What is broken or missing.
- `## Non-goals` — bulleted. What is explicitly out of scope.
- `## Verification` — operational checks; both subsections below required.
- `## Links` — `[[wikilink]]` form. Targets must resolve.

`anvil create issue` rejects a body missing any required H2.

## Optional: `## Acceptance criteria`

A prose checklist, if useful. **Not required** — `goal:` owns the terminal predicate and `## Verification` owns the runnable test-list, so AC no longer carries a unique job. Include it only when an unambiguous bulleted list aids the implementer; otherwise omit it.

## `## Verification` format

Two required subsections, each containing one or more fenced bash blocks whose lines are shell commands. Each command must exit 0 to count as passing. No DSL — the predicate lives inside the command itself (`grep -q`, `jq`, `test`, `[ ... = ... ]`).

### Direct (unit/integration)

Tests run against the dev tree / working copy: unit tests, integration tests, lint, type-check, schema-validate. Cheap to run, cheap to iterate.

```bash
go test ./internal/transition -run TestClaimAtomic
```

### Indirect (live smoke)

Live invocations against the built/installed/served artifact, proving the change works end-to-end. The check `completing-issue`'s Phase 4 build-and-install gate re-runs against the installed binary — direct passes here cannot mask install-path bugs.

Each predicate must exercise behaviour and assert on observed output or side-effects. Presence-only patterns exit 0 without touching runtime behaviour and must not be used as Indirect checks:

- `<cmd> --help | grep "feature"` — grepping help text proves the flag exists, not that it works.
- `test -f <path>` — proving a file was installed is not a behavioral check.
- `grep "pattern" <source-or-skill-file>` — grepping source proves the text is there, not that the artifact behaves correctly.

These are anti-patterns. Write predicates that invoke the artifact with real inputs and assert on the result.

**Worked example.** The `wait-for-pr.sh` issue used `scripts/wait-for-pr.sh --help | grep` as its Indirect check. That predicate passed even though `go:embed` had stripped the exec bit — so the installed script was non-executable (`permission denied`). Only `bash scripts/wait-for-pr.sh ... | jq -e` would have caught it because it actually runs the script. The rule: if the installed artifact is a shell script, invoke it (via `bash <script> ...`) and assert on its output; do not grep its help text or its source.

```bash
anvil transition issue test-fixture in-progress --owner test 2>&1 | grep -q "transitioned to in-progress"
[ "$(anvil show issue test-fixture --json | jq -r .status)" = "in-progress" ]
```

**Doc/skill-only changes.** When the change is purely a doc or skill update with no invocable binary artifact, assert on the rendered/installed content rather than the source tree: `anvil show skill <name> | grep -q "..."` exercises the install path (see `docs/skill-authoring.md`). Grepping the SKILL.md source file directly is still an anti-pattern — it skips the install step where the content could differ.

## Parsing rules

- Each `` ```bash `` block is one check: its lines run together as a single script, so state set on one line (`out=$(cmd)`) is visible to the next. Exit code of the block = exit code of its last command; 0 = PASS, non-zero = FAIL. Put the load-bearing assertion last (an earlier line's non-zero exit is not the block's verdict), or split genuinely independent assertions into separate blocks — each block is its own check.
- Comments and blank lines run as part of the script — they are not stripped, so heredocs stay intact.
- Multiple `` ```bash `` blocks in the same subsection are separate checks, run in order. State is **not** shared across blocks.
- The block opener must be exactly `` ```bash `` (with no trailing chars); other fence languages are not parsed as checks. A block's own content may contain nested `` ``` `` fences (e.g. a heredoc holding a mini issue doc) — fence depth is tracked, so only the outermost opener starts a check.
- Blocks run in the cwd the runner is invoked from — the worktree under test. Do **not** `cd` to an absolute main-checkout path; anchor with `$(git rev-parse --show-toplevel)` if you need the repo root.
- A subsection with no `` ```bash `` block is a validation failure — author at least one check or remove the subsection (and accept the validation reject from `anvil create issue`).

## Rename / migration verification

When an issue renames a symbol, table, layer, or identifier across a multi-package or multi-workspace repo, narrow single-package greps silently miss cross-package reference breaks and retired-name leaks.

**Scope repo-wide, not to the renamed package.** A grep scoped to `renamed-pkg/models` will miss callers in sibling workspaces. Use `git grep` (repo-root-relative; anchor the cwd as the Parsing-rules note above) or `grep -r`, or pipe through `find . -name '*.go'` — anything that spans every workspace:

```bash
# wrong: only covers one package
grep -r "OldName" burgh-data/models

# right: cross-package / repo-wide
git grep -r "OldName" -- '*.go'
```

**Account for names that survive the rename.** A retired spelling can remain valid in a surviving layer. Blanket-forbidding a token (`grep -rq "normalised"` exits non-zero if found) will flag correct surviving usages. Instead, assert that the old name is absent only where it should be absent, and that the canonical surviving usage exists where it should exist:

```bash
# wrong: forbids the token globally even where a layer still uses it legitimately
! git grep -q "normalised" -- '*.go'

# right: forbid it only where it was retired, and confirm it survives where it is still correct
! git grep -q "normalised" -- renamed-pkg/
git grep -q "normalised" -- surviving-layer/
```

Pair every absence check with a positive existence check: `! git grep -q "X" -- pkg/` also passes vacuously when `pkg/` is mistyped or empty — the exact false-green this section warns against.

## Why both subsections

Direct ("tests pass") can stay green while the feature is broken end-to-end — install path bug, wiring error, missing migration, regression in an adjacent verb. Indirect closes that gap by running the actual product, not the test harness. Refusing to author the indirect check is how regressions land in merged PRs.
