# Single-issue dispatch (model-tuned, stop at PR-opened)

Contract for delegating **one** issue to a subagent that runs on a cheaper model than the main agent, in an isolated context. The main agent keeps its model (e.g. Opus); the subagent does the implementation churn on Sonnet, so the churn never enters the main thread. Use this for a one-off completion — NOT the fleet. `dispatching-issue-fleet` owns N-parallel dispatch and deliberately keeps its own in-subagent review loop.

## Dispatch parameters (orchestrator side)

Fire one subagent via the Agent tool:

- `model: sonnet` — the cost lever. The subagent runs Sonnet; the main agent keeps its own model. (PR #94 proved Sonnet completes a well-specified issue to a clean, CI-green PR.)
- `effort` — **not settable here.** The Agent tool takes no per-call effort parameter (anthropics/claude-code#25669), only `model`. For non-default effort, set session `/effort <level>` before dispatching; because this is a foreground dispatch the parked main agent is unaffected in practice, so the session-wide setting effectively bites only the worker (reset `/effort` afterward). Add an `effort` dispatch parameter here once #25669 lands.
- `subagent_type: general-purpose` — needs Bash, Read, Edit, Write, and the Skill tool (to fire `completing-issue`).
- **Foreground**, not background — permission prompts pass through to the human. A background subagent auto-denies un-pre-approved calls (`gh pr create`, `git`, `just install`) and stalls.

Fill these fields into the prompt below before sending:

- `<issue-id>` — the anvil issue id the subagent owns.
- `<worktree-path>` — absolute path the subagent works in.
- `<branch>` — the branch the worktree is on (e.g. `anvil/<slug>`).
- `<declared-files>` — best estimate of the files the issue touches; the subagent confirms by grep and fires a scope-change Blocker if reality exceeds it.

## Prompt template

> You are a single-issue subagent in this repo. You own ONE issue and STOP at PR-opened. You have no prior conversation context; this prompt is the only contract.
>
> **Scope (narrower than the fleet contract).** Drive `completing-issue` against `<issue-id>` to an opened PR, then HALT. Do NOT invoke `responding-to-pr-review`. Do NOT poll, monitor, or wait on CI or CodeRabbit. The moment `gh pr create` returns a url, emit it and terminate — the human runs review separately. This stop-at-PR-opened rule is the whole point: the fleet's review-respond polling loop is where one-off subagents hang.
>
> **Worktree.** Work in `<worktree-path>` on `<branch>`; cut it per `docs/worktree-workflow.md` if it does not exist. PRE-EDIT INVARIANT: before every edit, `git rev-parse --show-toplevel` must equal `<worktree-path>` exactly — else halt with `Blocker: write-outside-worktree (toplevel=<actual>)`, not self-correctable.
>
> **Declared files.** `<declared-files>` (orchestrator estimate). Grep to confirm before editing; if the real change materially exceeds this set, halt with `Blocker: scope-change <metric>=<observed> vs <declared> — <cause>` rather than silently expanding.
>
> **Project rules.** Read `CLAUDE.md`, `AGENTS.md`, and `docs/` (go-conventions, code-design, agent-cli-principles, guardrails) — they are NOT auto-injected for you. Follow completing-issue's phases including the `just install` smoke gate before `gh pr create`.
>
> **Forbidden calls.** Never `gh pr merge`, `git worktree remove`, `anvil transition resolved`, or `anvil transition abandoned` — the human owns those.
>
> **Return contract.** Your LAST LINE, alone, is exactly one of: the PR url (`https://github.com/.../pull/<n>`) or `Blocker: <one line>`. Immediately before it, print: `Forbidden-call audit: gh pr merge=not-called, git worktree remove=not-called, anvil transition resolved=not-called, anvil transition abandoned=not-called.` No narrative tail, no "waiting" / "let me check".

## Why stop at PR-opened

`completing-issue`'s own contract ends at PR-opened (its Phase 5 surfaces the url and stops). The review-respond loop is a fleet-layer addition with dedicated anti-hang machinery; bolting it onto a one-off dispatch reintroduces the narrative-as-final-output stall — the subagent narrates "waiting for monitor events" instead of returning. The human picks up review after the url returns.
