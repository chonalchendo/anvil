# Anvil Issue Body Spec

Body-section contract for anvil issues. Authored by `anvil:writing-issue`; consumed by `anvil:completing-issue` and `skills/completing-issue/scripts/run-verification.sh`.

For frontmatter, see [vault-schemas.md](vault-schemas.md#issue).

## Required body sections

- `## Problem` — one paragraph. What is broken or missing.
- `## Acceptance criteria` — bulleted. Each testable without ambiguity.
- `## Non-goals` — bulleted. What is explicitly out of scope.
- `## Verification` — operational checks; both subsections below required.
- `## Links` — `[[wikilink]]` form. Targets must resolve.

`anvil create issue` rejects a body missing any required H2.

## `## Verification` format

Two required subsections, each containing one or more fenced bash blocks whose lines are shell commands. Each command must exit 0 to count as passing. No DSL — the predicate lives inside the command itself (`grep -q`, `jq`, `test`, `[ ... = ... ]`).

### Direct (unit/integration)

Tests run against the dev tree / working copy: unit tests, integration tests, lint, type-check, schema-validate. Cheap to run, cheap to iterate.

```bash
go test ./internal/transition -run TestClaimAtomic
```

### Indirect (live smoke)

Live invocations against the built/installed/served artifact, proving the change works end-to-end. The check `anvil:completing-issue`'s Phase 4 build-and-install gate re-runs against the installed binary — direct passes here cannot mask install-path bugs.

```bash
anvil transition issue test-fixture in-progress --owner test 2>&1 | grep -q "transitioned to in-progress"
[ "$(anvil show issue test-fixture --json | jq -r .status)" = "in-progress" ]
```

## Parsing rules

- Each line inside a `` ```bash `` … `` ``` `` block is one check. Run via `bash -c`; exit 0 = PASS, non-zero = FAIL.
- Lines starting with `#` and blank lines are ignored.
- Multiple fenced blocks in the same subsection are concatenated in order.
- The fence opener must be exactly `` ```bash `` (with no trailing chars); other fence languages are not parsed as checks.
- A subsection with no executable lines is a validation failure — author at least one check or remove the subsection (and accept the validation reject from `anvil create issue`).

## Why both subsections

Direct ("tests pass") can stay green while the feature is broken end-to-end — install path bug, wiring error, missing migration, regression in an adjacent verb. Indirect closes that gap by running the actual product, not the test harness. Refusing to author the indirect check is how regressions land in merged PRs.
