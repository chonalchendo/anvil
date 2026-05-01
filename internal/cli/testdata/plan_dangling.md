---
type: plan
id: ANV-142
slug: streaming-token-counter
title: "Stream-aware token counter"
created: "2026-04-30"
updated: "2026-04-30"
status: draft
plan_version: 1
issue: "[[issue.ANV-142]]"
tasks:
  - id: T1
    title: "Define TokenUsage type"
    kind: tdd
    files: ["a.go", "a_test.go"]
    depends_on: []
    verify: "go test ./..."
  - id: T2
    title: "Streaming reader"
    kind: tdd
    files: ["b.go", "b_test.go"]
    depends_on: [T9]
    verify: "go test ./..."
---

## Goal

Stream-aware token counter for real-time billing and observability.

## Task: T1

Define the TokenUsage type in a.go. Write the RED test in a_test.go first to
assert that zero-value fields are sane and that accumulation arithmetic is
correct. The type must track prompt tokens, completion tokens, and total tokens
as separate fields with an Add method. Run "go test ./..." to confirm RED,
implement the type, then run again to confirm GREEN. Commit once verify passes.

## Task: T2

Implement the streaming reader in b.go. Write the RED test in b_test.go first
to assert that the reader correctly accumulates tokens across multiple chunks
and returns an error on malformed input. The reader wraps an io.Reader and
collects usage metadata emitted as JSON lines. Run "go test ./..." to confirm
RED, implement the reader, then run again to confirm GREEN. Commit once verify
passes. Ensure the reader closes the underlying source on completion.
