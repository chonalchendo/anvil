# Single-issue dispatch (model-tuned, stop at PR-opened)

Delegate **one** issue to a worker on a cheaper model than the main agent, in an isolated context. The main agent keeps Opus; the worker does the implementation churn on Sonnet, so it never enters the main thread. One-off completion only — `dispatching-issue-fleet` owns N-parallel dispatch and keeps its own in-subagent review loop.

The worker's model, effort, allowed tools, and preloaded `completing-issue` skill — plus the invariant orchestration contract (stop-at-PR-opened with no review loop, pre-edit worktree invariant, scope-change Blocker, forbidden-call audit, structured return line) — all live in the bundled agent `anvil/agents/anvil-issue-worker.md`, deployed to `~/.claude/agents/anvil-issue-worker.md` by `anvil install agents`. Tune the cost levers (`model`, `effort`) by editing the bundled source, then `just install` && `anvil install agents`; nothing here re-templates them per call.

**One-time caveat: restart first.** A freshly-deployed or -edited `~/.claude/agents/anvil-issue-worker.md` is NOT dispatchable until the next session restart — the Agent tool enumerates `subagent_type` values at session start. If `subagent_type: anvil-issue-worker` errors with "Agent type not found", restart the session once, then dispatch.

## Dispatch

Fire one **foreground** subagent via the Agent tool with `subagent_type: anvil-issue-worker`. Foreground (not background) so permission prompts for `gh pr create`, `git`, and `just install` reach the human; a background worker auto-denies them and stalls.

Fill these per-call values into the dispatch prompt — the agent file carries the rest:

- `<issue-id>` — the anvil issue the worker owns.
- `<worktree-path>` — absolute path the worker edits in. **Claim and cut it before dispatch**, one atomic call: `anvil transition issue <id> in-progress --owner <name> --cut-worktree` claims the issue `in-progress` and emits the worktree path. The worker arrives pre-claimed and skips its own Phase 0 claim — claiming on the orchestrator is why the issue never stays `open`.
- `<branch>` — the branch the worktree is on (e.g. `anvil/<slug>`).
- `<declared-files>` — best estimate of the files the issue touches; the worker grep-confirms and fires a scope-change Blocker if reality exceeds it.

Prompt body:

> Complete anvil issue `<issue-id>`. Worktree: `<worktree-path>` on branch `<branch>`. Declared files (estimate, grep to confirm): `<declared-files>`.

## After the worker returns — the chain is yours

The worker halts at PR-opened by contract: in-subagent review polling is where one-off workers hang, and a subagent cannot dispatch the reviewer sub-subagent anyway. So the rest of the chain runs **in your main thread**, never delegated back into a worker. A `Blocker:` line instead of a PR url ends the run here — record it and stop.

1. **Review** — fire `reviewing-pr` on the returned PR. It dispatches a fresh `anvil-pr-reviewer` and hands back structured findings.
2. **Respond** — fire `responding-to-pr-review` with that report. Every finding gets an outcome: apply, skip-with-reason, or push-back.
3. **Merge** — the human decides, per PR. On approval: `anvil transition issue <id> resolved --land-pr <n>`, which gates on mergeable + CI-green, merges, verifies, removes the worktree, and resolves in one call.

Steps 1 and 2 are not optional politeness — they are the independent-review and human-merge gates, the named defense against dispatch shipping unreviewed work. Skipping them because CI is green defeats the point of dispatching at all.
