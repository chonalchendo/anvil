# Agent-Friendly CLI Principles

Agents are first-class CLI consumers — they call `anvil` dozens of times per session with no human in the loop. The eight rules below extend the design-rules line in `docs/system-design/cli-substrate.md` ("boring, no interactive prompts, JSON output behind `--json`, stdout for content, stderr for diagnostics, meaningful exit codes") with concrete guidance for agent-facing verbs. Source: compound-engineering, *Building Agent-Friendly CLIs: Practical Principles*.

## 1. Non-interactive automation paths

Never prompt. Never require a TTY. Never depend on `$EDITOR`. Every `create` and `set` verb must accept all inputs as flags; any missing required flag is a validation error on stderr (exit 1), not an interactive prompt. If a value can't be inferred and wasn't supplied, refuse with a clear error.

## 2. Structured output

`--json` is mandatory on every read-shape verb: `list`, `show`, `where`, `inbox list`, `inbox show`, `project list`, `project current`. JSON output must be stable across patch versions and documented in the verb's `--help`. Human-readable output is the default; `--json` is the machine contract. Do not intermix prose and JSON on stdout.

## 3. Progressive help discovery

`--help` on every verb shows: required flags, optional flags with defaults, at least one usage example, and a pointer to deeper docs when they exist. Subcommands are discoverable from their parent (`anvil --help` lists top-level groups; `anvil inbox --help` lists `add | list | show | promote`). An agent that has never seen a verb can bootstrap from `--help` alone.

## 4. Precise constraint language in --help

Name the enforced bound, not a range. Write `max 120 chars` when only the upper bound is checked; write `min N` when only the lower bound is enforced. Reserve `M-N` ranges for genuinely closed intervals where both ends matter (e.g., `--port 1024-65535`). A help string that reads `1-120 chars` forces an agent to verify which end is enforced — a wasted round-trip. When the constraint is "required and capped", say `required, max 120 chars`.

## 5. Actionable errors

Validation failures from `create` and `set` print three things: the offending field, the set of valid values pulled from the schema, and a copy-pasteable corrected invocation. Example: `anvil create issue --priority urgent` should produce:

```
error: invalid value "urgent" for --priority
  valid values: low, medium, high, critical
  corrected:    anvil create issue --priority high [other flags]
```

Use sentinel errors from `internal/cli/errors.go` so the error shape is consistent across verbs.

## 6. Safe retries (idempotence)

`anvil create <type> --slug X` with identical content is a no-op: print the existing artifact ID and exit 0. Content drift (same slug, different field values) is an error unless `--update` is passed. Re-running `anvil link <type> <id> --to <type> <id>` with the same pair is a no-op. Agents retry on transient failures; idempotence prevents duplicate artifacts.

## 7. Composability

CLI output flows cleanly to downstream tools. All content goes to stdout; all diagnostics go to stderr. Use `cmd.Println` / `cmd.PrintErrln`, never `fmt.Println` (cobra respects output redirection; `fmt` doesn't). Exit codes are meaningful: 0 success, 1 validation/user error, 2 internal error. `--json` output pipes cleanly to `jq` — no ANSI codes, no progress noise, no preamble prose.

## 8. Bounded responses

`anvil list <type>` defaults to `--limit 50`. When the result set is truncated, print a narrowing hint on stderr: available filter flags and how to raise the limit. Example:

```
(showing 50 of 312 issues — narrow with --status, --milestone, or --limit N)
```

`anvil show` on a type with a large body field should print a summary view by default and accept `--body` for the artifact body (capped). Unbounded output breaks agent context budgets.

---

## Verb-type cheatsheet

| Shape    | Dominant rules                                    |
| -------- | ------------------------------------------------- |
| `read`   | `--json` mandatory, bounded by default, composable stdout |
| `mutate` | Non-interactive, actionable errors, idempotent    |
| `bulk`   | Bounded output, narrowing hints, safe to retry    |

---

Outstanding gaps tracked under `## Agent-CLI readiness` in `docs/system-design/roadmap.md`.
