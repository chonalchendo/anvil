# Replay

A replay re-runs a resolved issue from the base its PR started on. The result shows what the current skills and agents cost and whether they still pass the issue's verification.

## The set

Find candidates with `anvil list issue --status resolved --json --limit 50`. Keep each id where this passes:

```bash
anvil show issue <id> --json --no-body | jq -e '.cost_tokens and ((.external_links // []) | any(test("/pull/")))'
```

Take five by default. Skip an issue with no cost record: it has nothing to compare.

## The loop

For each issue, one at a time:

1. `anvil replay <id> --json` prints `{worktree, base}` and cuts a `replay/<slug>` branch there. A refusal names the cause: not resolved, no merged PR, gh missing or failing.
2. Dispatch `subagent_type: anvil-issue-worker` with a prompt that starts `Replay:`:

   > Replay: anvil issue `<issue-id>`. Worktree: `<worktree>`. Branch: `replay/<slug>`.

   Give no PR url, no review text and no learnings about this issue. The worker returns a commit sha as its last line.
3. Read the worker's token total, then from inside `<worktree>` run `anvil verify <id> --replay --tokens <n>`. It appends a Replay section to the issue and changes no landed field.
4. `anvil replay <id> --remove` removes the worktree and its branch. Run it also after a failed or halted replay.

Run `anvil replay` and `anvil verify --replay` yourself. Never run `git worktree remove`.

## Triage a red replay

A red replay does not count until you triage it. Read the failed check first.

- Environment drift (a tool, a path or a network call changed since the landing): record it and drop the replay from the comparison.
- A worker that could not follow a skill or an agent rule: this is a finding. Name the rule.
- A verification predicate that never fit the code: this is an issue-authoring finding.

The worker reads today's designs. A design edited after the issue's `claimed_at` may hold the landed answer: flag such a replay in the report.

## Report

List one line per replay: the issue id, the verdict, and the replay cost beside the landed cost (`diff`, `files`, `tokens`). State the triage result for each red line.
