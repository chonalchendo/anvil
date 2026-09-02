# Worktree Workflow

Applies to every task in the anvil repo — non-negotiable.

## Cutting a Worktree

**Issue-backed work:** skip the manual sequence — `anvil transition issue <id> in-progress --owner <name> --cut-worktree` claims, fetches, and branches from `origin/HEAD` in one call, emitting the worktree path. It resolves the repo from the issue's `project` (`~/Development/<project>`, or the cwd repo when that *is* the project), never from whatever repo you happen to be standing in, and refuses with `cut_worktree_repo_unresolved` when neither resolves. The manual sequence below remains for issueless tasks.

A repo-root `.anvil-worktree-carry` file (one repo-relative path per line, `#` comments skipped) declares untracked files — typically `.env` credentials the smoke-test gate needs — that `--cut-worktree` copies into each freshly cut worktree; a declared path missing from the source checkout refuses the cut with `cut_worktree_carry_missing`, and an absolute or repo-escaping entry with `cut_worktree_carry_invalid`. Declared paths must be gitignored: a carried file that is not ignored makes `--land-pr`'s post-merge `git worktree remove` refuse (`contains modified or untracked files`), stranding the issue in-progress.

After carry lands on a freshly cut worktree, `--cut-worktree` runs a repo-root `.anvil-worktree-hook` when present — a tracked, executable escape hatch for state carry can't express, such as deriving a worktree-local `.env` from a read-only agent principal rather than copying the repo's own. It runs synchronously with cwd at the repo root and the absolute worktree path as `$1`, so it must finish (and spawn nothing that outlives it) before the cut returns — a lingering process wedges a later `git worktree remove`. A missing hook is a silent no-op; present but not executable refuses with `cut_worktree_hook_not_executable`; a non-zero exit refuses with `cut_worktree_hook_failed` (carrying the hook's stderr tail) and removes the worktree and branch the cut just made, so a failing hook orphans nothing; a hook still running after 30s refuses with `cut_worktree_hook_timeout` and runs the same cleanup. A reused worktree skips the hook, same as carry. Anything the hook writes into the worktree must be gitignored, for the same reason carry's declared paths must be: a non-ignored write makes `--land-pr`'s post-merge `git worktree remove` refuse, even on a hook that succeeds.

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

- If the user said something equivalent to "monitor until merge" / "wait for it to land" / "babysit the PR", that authorizes *monitoring*, not unprompted merging: poll until review + CI are green and the PR is mergeable, then surface it for the human's per-PR go. The agent lands only on that explicit approval — `responding-to-pr-review`'s merge gate for skill-driven PRs, the [Post-merge cleanup](#post-merge-cleanup-sequence-matters) sequence otherwise. Babysit is not standing approval to merge.
- Otherwise: schedule **at most one** wakeup (≥ 2 h), check once, then stop and surface "awaiting your call" rather than looping.
- Exception: a blocking CI job expected to finish in ≤ 5 min may be checked immediately once without a scheduled wakeup.

## Workflow Summary

Cut worktree → implement + commit → smoke-test gate → `gh pr create` → `reviewing-pr` (subagent review) + CI → user approval → land (`anvil transition issue <id> resolved --land-pr <pr>` for issue-backed PRs; manual sequence otherwise — see [Post-merge cleanup](#post-merge-cleanup-sequence-matters)). The independent review catches what unit tests miss — part of the verification budget, not optional.
