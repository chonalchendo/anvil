# Worktree Workflow

Applies to every task in the anvil repo — non-negotiable.

## Cutting a Worktree

```bash
git -C ~/Development/anvil worktree add ~/Development/anvil-worktrees/<slug> -b anvil/<slug>
cd ~/Development/anvil-worktrees/<slug>
```

After merge: `git -C ~/Development/anvil worktree remove ~/Development/anvil-worktrees/<slug>`.

Never `git checkout -b` or commit directly on `master` — parallel sessions collide, CodeRabbit gets no review pass, work accumulates unreviewed.

## Gate: Smoke-Test Before PR

Before `gh pr create` or claiming done, drive the change through the installed binary against a real vault.

1. `go install ./cmd/anvil`.
2. Confirm freshness: `which anvil` resolves to `$(go env GOPATH)/bin/anvil`. Cross-check `anvil --version` ends in the short sha of `git rev-parse --short HEAD` (building from a worktree emits `dev` with no sha — run from the main checkout if you need the sha confirmed).
3. Invoke the new verb, re-trigger the changed error, or read the new skill phase end-to-end.
4. Compare output against acceptance criteria.
5. Any failure (broken commands in error hints, schema-inconsistent JSON, oversized output, blank fields) is a regression — fix before resolving.

Unit tests assert *some* string appears in output; they don't assert it's runnable, schema-consistent, or usable on 40 KB real-vault artifacts. Only live invocation catches that.

## Waiting on human-gated PR events

Do not poll PRs on a fixed interval — each wakeup past the 300 s prompt-cache window pays a full context reload.

- If the user said something equivalent to "monitor until merge" / "wait for it to land" / "babysit the PR", treat that as standing approval: once CodeRabbit + CI are green and the PR is mergeable, merge and remove the worktree without a further prompt.
- Otherwise: schedule **at most one** wakeup (≥ 2 h), check once, then stop and surface "awaiting your call" rather than looping.
- Exception: a blocking CI job expected to finish in ≤ 5 min may be checked immediately once without a scheduled wakeup.

## Workflow Summary

Cut worktree → implement + commit → smoke-test gate → `gh pr create` → wait for CodeRabbit + user approval → remove worktree after merge. CodeRabbit catches what unit tests miss — part of the verification budget, not optional.
