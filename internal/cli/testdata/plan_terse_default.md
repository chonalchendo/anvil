---
type: plan
id: ANV-999
slug: terse-default
title: "Terse default fixture"
description: "Fixture pinning terse plan-task default."
created: "2026-05-14"
updated: "2026-05-14"
status: draft
plan_version: 1
issue: "[[issue.ANV-999]]"
tags: [type/plan, domain/dev-tools]
tasks:
  - id: T1
    title: "Terse task"
    kind: tdd
    files: ["a.go", "a_test.go"]
    depends_on: []
    verify: "go test ./..."
    success_criteria:
      - "Add method returns correct sum"
      - "zero-value fields are sane"
---

## Task: T1

### Context the executor needs

This prose preamble is the body content the terse mode must omit. It paraphrases
the structured fields above and would inflate per-task fetch cost during a plan
walk if included by default. The Iron Law of test-as-spec means the executor
only needs the structured fields to know success/failure.
