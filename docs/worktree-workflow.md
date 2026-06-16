# Worktree Workflow

Applies to every task in the anvil repo — non-negotiable.

## Cutting a Worktree

**Issue-backed work:** skip the manual sequence — `anvil transition issue <id> in-progress --owner <name> --cut-worktree` claims, fetches, and branches from `origin/HEAD` in one call, emitting the worktree path. The manual sequence below remains for issueless tasks.

```bash
git -C ~/Development/anvil fetch origin
git -C ~/Development/anvil worktree add ~/Development/anvil-worktrees/<slug> -b anvil/<slug> origin/master
cd ~/Development/anvil-worktrees/<slug>
```

Never `git checkout -b` or commit directly on `master` — parallel sessions collide, no review pass happens, work accumulates unreviewed.

## Post-merge cleanup (sequence matters)

**Issue-backed PRs:** skip the manual sequence — `anvil transition issue <id> resolved --land-pr <pr>` performs gate → squash-merge → MERGED-verify → worktree remove → branch delete (local + remote) and resolves the issue in the same call. The merge runs *before* the worktree is removed so the verb survives being invoked from inside the worktree it cleans up; branch deletion follows removal because git refuses to delete a branch a worktree still references. The manual two-step below remains for issueless PRs only.

`gh pr merge --delete-branch` refuses the local-branch delete while the worktree is still checked out (`cannot delete branch 'anvil/<slug>' used by worktree at ...`). For the manual path, remove the worktree **first**:

```bash
git -C ~/Development/anvil worktree remove ~/Development/anvil-worktrees/<slug>
gh pr merge <pr> --squash --delete-branch
```

Or drop `--delete-branch` and delete the local branch yourself after `worktree remove`. Never chain `worktree remove` after `gh pr merge --delete-branch` — the merge succeeds, the branch delete fails silently inside `gh`, and you finish with a stale local branch.

## Gate: Smoke-Test Before PR

Before `gh pr create` or claiming done, drive the change through the installed binary against a real vault.

1. `just install-local` — builds a version-stamped binary into this worktree's `./bin/anvil` (`GOBIN=$PWD/bin`), **not** the shared global `$(go env GOPATH)/bin`. Parallel worktrees therefore install to distinct files and never clobber each other — required because `dispatching-issue-fleet` runs workers concurrently and a shared global target makes the version cross-check below flake (`just install`, the global interactive install, races here). Invoke the gate's binary as `./bin/anvil`; no PATH-shadow check is needed since you call it by path.
2. Cross-check `./bin/anvil --version` ends in the short sha of `git rev-parse --short HEAD` (with a `-dirty` suffix if the tree has uncommitted changes). `install-local` injects the sha via `-ldflags` so worktree builds stamp correctly — Go's `buildvcs` drops VCS metadata for worktrees (golang/go#58300), so `./bin/anvil --version` reporting bare `dev` means the build bypassed `install-local`.
3. `just lint` — runs `golangci-lint` with the same ruleset CI uses. A lint failure here is a CI failure; fix before opening the PR. (Stale lint cache from a removed worktree can block this — run `golangci-lint cache clean` if lint exits with a spurious cache error.)
4. Invoke the new verb (`./bin/anvil <verb>`), re-trigger the changed error, or read the new skill phase end-to-end.
5. Compare output against acceptance criteria.
6. Any failure (broken commands in error hints, schema-inconsistent JSON, oversized output, blank fields) is a regression — fix before resolving.

Unit tests assert *some* string appears in output; they don't assert it's runnable, schema-consistent, or usable on 40 KB real-vault artifacts. Only live invocation catches that.

## Waiting on human-gated PR events

Do not poll PRs on a fixed interval — each wakeup past the 300 s prompt-cache window pays a full context reload.

- If the user said something equivalent to "monitor until merge" / "wait for it to land" / "babysit the PR", treat that as standing approval: once the review + CI are green and the PR is mergeable, run the [Post-merge cleanup](#post-merge-cleanup-sequence-matters) sequence without a further prompt.
- Otherwise: schedule **at most one** wakeup (≥ 2 h), check once, then stop and surface "awaiting your call" rather than looping.
- Exception: a blocking CI job expected to finish in ≤ 5 min may be checked immediately once without a scheduled wakeup.

## Workflow Summary

Cut worktree → implement + commit → smoke-test gate → `gh pr create` → `reviewing-pr` (subagent review) + CI → user approval → land (`anvil transition issue <id> resolved --land-pr <pr>` for issue-backed PRs; manual sequence otherwise — see [Post-merge cleanup](#post-merge-cleanup-sequence-matters)). The independent review catches what unit tests miss — part of the verification budget, not optional.
