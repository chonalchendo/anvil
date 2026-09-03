# Anvil Issue Body Spec

Body-section contract for anvil issues. Authored by `writing-issue`; consumed by `completing-issue` and `skills/completing-issue/scripts/run-verification.sh`.

For frontmatter, see [vault-schemas.md](vault-schemas.md#issue). The issue's terminal predicate lives in the required `goal:` frontmatter field — one sentence, ≤120 chars, naming what "done" means. The body sections below carry the detail.

## Required body sections

- `## Problem` — lead sentence naming what is wrong, then evidence, cause, direction, sequencing; `writing-issue` owns the shape. `anvil validate`/`create` flag a lead sentence over 25 words or containing a backtick as a `lead_sentence` warning — never blocking.
- `## Non-goals` — bulleted. What is explicitly out of scope.
- `## Verification` — operational checks; both subsections below required.
- `## Links` — `[[wikilink]]` form. Targets must resolve.

`anvil create issue` rejects a body missing any required H2.

## Optional: `## Acceptance criteria`

A prose checklist, if useful. **Not required** — `goal:` owns the terminal predicate and `## Verification` owns the runnable test-list, so AC no longer carries a unique job. Include it only when an unambiguous bulleted list aids the implementer; otherwise omit it.

## `## Verification` format

Two required subsections, each containing one or more fenced bash blocks whose lines are shell commands. Each command must exit 0 to count as passing. No DSL — the predicate lives inside the command itself (`grep -q`, `jq`, `test`, `[ ... = ... ]`).

**Feasibility is enforced mechanically, not just advised.** `anvil create issue` and `anvil promote --as issue` actually run every `### Direct`/`### Indirect` fenced-bash block in the authoring environment and judge the issue by the exit status — so a predicate that has never run cannot ship as a green Iron Law gate. Each block runs under `set -e` (lines share state; the first failing line fails the block), capped at 60s and killed by process group.

The verdict is asymmetric, because the two subsections are in opposite states at authoring time:

| | exit 0 | 126 / 127 | other non-zero | timeout |
|---|---|---|---|---|
| **Indirect** | refused — already passes, so it cannot discriminate fixed from broken | refused — unrunnable | **accepted** (the healthy shape) | refused — unclassifiable |
| **Direct** | accepted | refused — unrunnable | accepted | accepted, unjudged |

An Indirect block asserts POST-fix behaviour, so it is *expected* to be red until the fix lands; exit 0 is the false-green this gate exists to kill. A Direct block is usually the repo's existing suite, green already, so only runnability is checked there.

**The gate is neither read-only nor retry-safe.** These are author-supplied shell commands running unsandboxed with your privileges, cwd and environment. Whatever a block does — rebuild `bin/anvil`, write a marker file, hit the network — persists even when the create is refused and rolled back, so a retry re-runs it. Keep blocks to the one command that proves the predicate. `--skip-verify-predicates` is the escape hatch; using it ships an unproven predicate.

### Direct (unit/integration)

Tests run against the dev tree / working copy: unit tests, integration tests, lint, type-check, schema-validate. Cheap to run, cheap to iterate.

```bash
go test ./internal/transition -run TestClaimAtomic
```

### Indirect (live smoke)

Live invocations against the built/installed/served artifact, proving the change works end-to-end. The check `completing-issue`'s Phase 4 build-and-install gate re-runs against the installed binary — direct passes here cannot mask install-path bugs. Build with `just install-local` and invoke `./bin/anvil` — a bare `anvil` resolves the globally installed binary, i.e. the base branch, so the predicate would smoke the wrong build even from the right worktree.

Each predicate must exercise behaviour and assert on observed output or side-effects. Presence-only patterns exit 0 without touching runtime behaviour and must not be used as Indirect checks:

- `<cmd> --help | grep "feature"` — grepping help text proves the flag exists, not that it works.
- `test -f <path>` — proving a file was installed is not a behavioral check.
- `grep "pattern" <source-or-skill-file>` — grepping source proves the text is there, not that the artifact behaves correctly.
- `<side-effect cmd> | grep -q "..."` — `grep -q` (or `head`) exits on first match and SIGPIPE-kills the producer mid-run, so the predicate passes while the side effect never completed. Capture first, then assert: `o=$(mktemp); <cmd> > "$o" 2>&1; grep -q "..." "$o"`.

These are anti-patterns. Write predicates that invoke the artifact with real inputs and assert on the result.

**Worked example.** The `wait-for-pr.sh` issue used `scripts/wait-for-pr.sh --help | grep` as its Indirect check. That predicate passed even though `go:embed` had stripped the exec bit — so the installed script was non-executable (`permission denied`). Only `bash scripts/wait-for-pr.sh ... | jq -e` would have caught it because it actually runs the script. The rule: if the installed artifact is a shell script, invoke it (via `bash <script> ...`) and assert on its output; do not grep its help text or its source.

```bash
o=$(mktemp)
anvil transition issue test-fixture in-progress --owner test > "$o" 2>&1
grep -q "transitioned to in-progress" "$o"
[ "$(anvil show issue test-fixture --json | jq -r .status)" = "in-progress" ]
```

**Doc/skill-only changes.** When the change is purely a doc or skill update with no invocable binary artifact, assert on the rendered/installed content rather than the source tree: `anvil show skill <name> | grep -q "..."` exercises the install path (see `docs/skill-authoring.md`). Grepping the SKILL.md source file directly is still an anti-pattern — it skips the install step where the content could differ.

## Universal predicate bars

`writing-issue` Phase 4 writes `### Direct`/`### Indirect` from the governing contract when one exists. No contract governs → every predicate, contract-drawn or not, still satisfies these bars:

- **Same code path** — the predicate travels the real system's path, not a proxy/metadata path that happens to be green (a dev check that can't reach the prod registry the goal lives in doesn't verify the goal).
- **Exercise, not presence** — assert on behaviour, never that a source file contains a string; the carve-out is a doc/skill-only change, which greps the *built/installed* artifact, never the source tree.
- **Create the unmet condition first** — prescribe a pre-step so the check fails before the change and passes after (a `max(A)==max(B)` freshness check is false-green when prior state already aligned both sides).
- **Anchor structurally** — assert against parsed structure (`jq` path, a typed field, an equality), not a bare substring grep.
- **The goal's own measure** — when `goal:` names a count, rate, or corpus state, the Indirect block computes that measure after its stated pre-step, never a fixture proxy. A prose "post-merge: re-measure" under the block is the tell that the goal is unverified.

## Parsing rules

- Each `` ```bash `` block is one check: its lines run together as a single script under `set -e`, so state set on one line (`out=$(cmd)`) is visible to the next, and the block FAILS on the **first** line that exits non-zero (all lines pass = PASS). Guard an intentionally-non-fatal line with `… || true`. **Positional caveat — negative assertions:** `set -e` exempts a `!`-negated command in every command position (`! c`, `x; ! c`, `a && ! c`, `for …; do ! c; done`, `if a; then ! c; fi`, `{ ! c; }`), so such a line fails the block only as its **last** line; anywhere earlier its failure is silently skipped. Write a non-last-line negative assertion as `if cmd; then exit 1; fi` — `anvil create issue` and `run-verification.sh` both refuse the vacuous form unrun, and `anvil validate --verification-stdin` flags it pre-flight. Pipelines are **not** under `pipefail`, so assert on a pipeline's final stage — a failing non-final stage (`cmd | grep …`) does not fail the block. Split genuinely independent assertions into separate blocks — each block is its own check.
- Comments and blank lines run as part of the script — they are not stripped, so heredocs stay intact.
- Multiple `` ```bash `` blocks in the same subsection are separate checks, run in order. State is **not** shared across blocks.
- The block opener must be exactly `` ```bash `` (with no trailing chars); other fence languages are not parsed as checks. A block's own content may contain nested `` ``` `` fences (e.g. a heredoc holding a mini issue doc) — fence depth is tracked, so only the outermost opener starts a check, and a `## `/`### ` line inside an open fence is block content, not a section boundary. An unterminated fence fails closed: the create is refused rather than running a partial block list.
- Blocks run in the cwd the runner is invoked from — the worktree under test. Do **not** reference an absolute main-checkout path in any command (`cd /Users/<user>/Development/<repo>`, `git -C ~/Development/<repo>`, `ls $HOME/Development/<repo>/…`); anchor with `$(git rev-parse --show-toplevel)` if you need the repo root. `anvil create issue`/`anvil promote --as issue` reject a hardcoded checkout path mechanically — a fleet worker dispatched to a worktree would otherwise silently verify the wrong tree; the vault-wide `anvil validate` scan deliberately does not (pre-existing issues predate the rule). Lint a predicate directly with `anvil validate --verification-stdin` (reads the bash text from stdin).
- A subsection with no `` ```bash `` block is a validation failure — author at least one check or remove the subsection (and accept the validation reject from `anvil create issue`).
- `anvil <verb> <subverb>…` invocations inside fenced blocks are validated at create/validate time against the live command tree: the full subcommand path is walked, so an unknown top-level verb (`anvil frobnicate`) **and** an unknown nested subcommand (`anvil project init` — `project` is real, `init` is not) both reject. Trailing flags/positionals are not checked.

## Rename / migration verification

When an issue renames a symbol, table, layer, or identifier across a multi-package or multi-workspace repo, narrow single-package greps silently miss cross-package reference breaks and retired-name leaks.

**Scope repo-wide, not to the renamed package.** A grep scoped to `renamed-pkg/models` will miss callers in sibling workspaces. Use `git grep` (repo-root-relative; anchor the cwd as the Parsing-rules note above) or `grep -r`, or pipe through `find . -name '*.go'` — anything that spans every workspace:

```bash
# wrong: only covers one package
grep -r "OldName" pkg/models

# right: cross-package / repo-wide
git grep -r "OldName" -- '*.go'
```

**Account for names that survive the rename.** A retired spelling can remain valid in a surviving layer. Blanket-forbidding a token (`grep -rq "normalised"` exits non-zero if found) will flag correct surviving usages. Instead, assert that the old name is absent only where it should be absent, and that the canonical surviving usage exists where it should exist:

```bash
# wrong: forbids the token globally even where a layer still uses it legitimately
if git grep -q "normalised" -- '*.go'; then exit 1; fi

# right: forbid it only where it was retired, and confirm it survives where it is still correct
if git grep -q "normalised" -- renamed-pkg/; then exit 1; fi
git grep -q "normalised" -- surviving-layer/
```

The absence check is written `if cmd; then exit 1; fi`, not `! cmd`, because it is not the block's last line — see the `set -e` caveat in *Parsing rules* above; both executors refuse the `!` form there.

Pair every absence check with a positive existence check: forbidding `X` in `pkg/` also passes vacuously when `pkg/` is mistyped or empty — the exact false-green this section warns against.

## Why both subsections

Direct ("tests pass") can stay green while the feature is broken end-to-end — install path bug, wiring error, missing migration, regression in an adjacent verb. Indirect closes that gap by running the actual product, not the test harness. Refusing to author the indirect check is how regressions land in merged PRs.
