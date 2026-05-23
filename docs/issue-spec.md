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

- Each line inside a `` ```bash `` … `` ``` `` block is one check. Run via `bash -c`; exit 0 = PASS, non-zero = FAIL.
- Lines starting with `#` and blank lines are ignored.
- Multiple fenced blocks in the same subsection are concatenated in order.
- The fence opener must be exactly `` ```bash `` (with no trailing chars); other fence languages are not parsed as checks.
- A subsection with no executable lines is a validation failure — author at least one check or remove the subsection (and accept the validation reject from `anvil create issue`).

## Why both subsections

Direct ("tests pass") can stay green while the feature is broken end-to-end — install path bug, wiring error, missing migration, regression in an adjacent verb. Indirect closes that gap by running the actual product, not the test harness. Refusing to author the indirect check is how regressions land in merged PRs.
