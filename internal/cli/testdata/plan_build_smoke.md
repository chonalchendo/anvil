---
type: plan
id: anvil.build-smoke
slug: build-smoke
title: "Build smoke fixture"
description: "Two-wave plan exercised by anvil build --dry-run in tests."
created: "2026-05-07"
updated: "2026-05-07"
status: draft
plan_version: 1
issue: "[[issue.anvil.build-smoke]]"
tags: [type/plan, domain/dev-tools]
tasks:
  - id: T1
    title: "Wave 0 task"
    kind: tdd
    model: claude-sonnet-4-6
    files: [a.go]
    depends_on: []
    verify: "true"
  - id: T2
    title: "Wave 1 task"
    kind: tdd
    model: claude-sonnet-4-6
    files: [b.go]
    depends_on: [T1]
    verify: "true"
---

## Task: T1

Smoke task one. This body has to be at least 200 characters long for the plan
validator to accept it, so we write a few sentences explaining what the agent
would do here in a real plan. Implement the wave-zero feature behind the
adapter contract and confirm verify exits 0.

## Task: T2

Smoke task two. This body has to be at least 200 characters long for the plan
validator to accept it, so we write a few sentences explaining what the agent
would do here in a real plan. Build on T1's output and confirm verify exits 0
once integration is complete.
