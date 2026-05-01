---
title: "Anvil system design — telemetry"
tags: [domain/dev-tools, type/system-design-shard]
---

## Telemetry

SQLite via `modernc.org/sqlite` — pure Go, no cgo. Rationale: cross-compiles cleanly to every release target, keeps `go install ...@latest` working without a C toolchain, and matches the single-binary distribution invariant. `mattn/go-sqlite3` is rejected on cgo grounds despite being faster; v0.1 telemetry volumes are tiny and the perf delta doesn't pay for the build complexity.

Adapter normalizers map per-CLI lines to `NormalizedEvent`. Sketches (signatures only):

```go
func normalizeClaude(line []byte) (NormalizedEvent, error) {
    // parse Claude Code NDJSON stream-json line into NormalizedEvent;
    // dedupe usage by assistant.message.id (multiple tool_use blocks share one).
    return NormalizedEvent{}, nil
}

func normalizeCodex(line []byte) (NormalizedEvent, error) {
    // parse Codex JSONL event line; treat `error` events matching ^Reconnecting
    // as Retry (non-terminal), only turn.failed / unprefixed error as Error.
    return NormalizedEvent{}, nil
}
```

Cost capture (sketch — full table in `design.md` § Telemetry collection):

| Field | Claude Code | Codex |
|---|---|---|
| `cost_usd` | terminal `result.total_cost_usd` (estimate on Max subscriptions) | not exposed via `exec`; quota only |
| `tokens` | last `result.usage` (deduped) | last `turn.completed.usage` |
| `api_key_source` | `system.init.apiKeySource` | n/a |

For Max users the cost is an estimate of API-equivalent rates, not actual billing. `anvil cost` surfaces that distinction honestly.
