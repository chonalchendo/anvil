# Single-batch dispatch (model-tuned, decisive path only)

Delegate a batch of already-shaped problem statements to a worker on a cheaper model than the main agent, in an isolated context. The main agent keeps Opus; the worker runs `writing-issue`'s decisive path per item on Sonnet, so the hundreds of authoring turns never enter the main thread. Only the decisive path delegates — a fuzzy thought still converges with the human in-session first (`writing-issue` Phase 0/1), then hands off as one already-shaped item.

The worker's model, effort, allowed tools, and preloaded `writing-issue` skill — plus the invariant orchestration contract (decisive-path-only, no learnings sub-dispatch, structured return line) — all live in the bundled `anvil-issue-author` agent definition, deployed to `~/.claude/agents/anvil-issue-author.md` by `anvil install agents`. Tune the cost levers (`model`, `effort`) by editing that bundled source in anvil's own checkout, rebuilding the binary with that project's build-and-install command, then re-running `anvil install agents`; nothing here re-templates them per call.

**One-time caveat: restart first.** A freshly-deployed or -edited `~/.claude/agents/anvil-issue-author.md` is NOT dispatchable until the next session restart — the Agent tool enumerates `subagent_type` values at session start. If `subagent_type: anvil-issue-author` errors with "Agent type not found", restart the session once, then dispatch.

## Dispatch

Fire one **foreground** subagent via the Agent tool with `subagent_type: anvil-issue-author`, one dispatch per milestone-scoped batch (mixing milestones dilutes the shared context the worker holds).

Fill these per-call values into the dispatch prompt — the agent file carries the rest:

- `<milestone-id>` — the single milestone every item in this batch links to.
- `<problem-statements>` — one shaped entry per item: a problem sentence, a goal (one-sentence terminal predicate), a severity, and any domain tag hint. Each must already be decisive (`writing-issue` Phase 0's bar) — the orchestrator converged any fuzzy ones before batching.
- `<learnings-gist>` — the prior-learnings summary the orchestrator already gathered for this batch (the worker cannot sub-dispatch `anvil-learnings-researcher`); empty if genuinely nothing relevant surfaced.

Prompt body:

> Author issues against milestone `<milestone-id>` for these problem statements: `<problem-statements>`. Prior learnings gist: `<learnings-gist>`.

## After the worker returns

The worker's last line is one created issue id per line, or `Blocker: <one line>` per item that couldn't clear the feasibility or milestone gate. Fold successes into the batch record; a blocker is yours to resolve or drop, same as any other authoring failure — the worker never guesses past one.
