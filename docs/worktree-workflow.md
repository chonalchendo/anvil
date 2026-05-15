# Worktree Workflow

Applies to every task in the anvil repo — non-negotiable.

## Cutting a Worktree

```bash
git -C ~/Development/anvil worktree add ~/Development/anvil-worktrees/<slug> -b anvil/<slug>
cd ~/Development/anvil-worktrees/<slug>
```

Never `git checkout -b` or commit directly on `master` — parallel sessions collide, CodeRabbit gets no review pass, work accumulates unreviewed.

## Post-merge cleanup (sequence matters)

`gh pr merge --delete-branch` refuses the local-branch delete while the worktree is still checked out (`cannot delete branch 'anvil/<slug>' used by worktree at ...`). Remove the worktree **first**:

```bash
git -C ~/Development/anvil worktree remove ~/Development/anvil-worktrees/<slug>
gh pr merge <pr> --squash --delete-branch
```

Or drop `--delete-branch` and delete the local branch yourself after `worktree remove`. Never chain `worktree remove` after `gh pr merge --delete-branch` — the merge succeeds, the branch delete fails silently inside `gh`, and you finish with a stale local branch.

## Gate: Smoke-Test Before PR

Before `gh pr create` or claiming done, drive the change through the installed binary against a real vault.

1. `just install` — runs `go install ./cmd/anvil` then asserts the `anvil` on `PATH` is the just-installed binary (same inode as `$(go env GOPATH)/bin/anvil`). Exits non-zero if a stale binary shadows it; fix PATH or the symlink before continuing. If your shell has cached an old path, run `hash -r`.
2. Cross-check `anvil --version` ends in the short sha of `git rev-parse --short HEAD` (with a `-dirty` suffix if the tree has uncommitted changes). `just install` injects the sha via `-ldflags` so worktree builds stamp correctly — Go's `buildvcs` drops VCS metadata for worktrees (golang/go#58300), so a bare `anvil --version` reporting `dev` means the install bypassed `just install`.
3. Invoke the new verb, re-trigger the changed error, or read the new skill phase end-to-end.
4. Compare output against acceptance criteria.
5. Any failure (broken commands in error hints, schema-inconsistent JSON, oversized output, blank fields) is a regression — fix before resolving.

Unit tests assert *some* string appears in output; they don't assert it's runnable, schema-consistent, or usable on 40 KB real-vault artifacts. Only live invocation catches that.

## Waiting on human-gated PR events

Do not poll PRs on a fixed interval — each wakeup past the 300 s prompt-cache window pays a full context reload.

- If the user said something equivalent to "monitor until merge" / "wait for it to land" / "babysit the PR", treat that as standing approval: once CodeRabbit + CI are green and the PR is mergeable, run the [Post-merge cleanup](#post-merge-cleanup-sequence-matters) sequence without a further prompt.
- Otherwise: schedule **at most one** wakeup (≥ 2 h), check once, then stop and surface "awaiting your call" rather than looping.
- Exception: a blocking CI job expected to finish in ≤ 5 min may be checked immediately once without a scheduled wakeup.

## Workflow Summary

Cut worktree → implement + commit → smoke-test gate → `gh pr create` → wait for CodeRabbit + user approval → remove worktree → `gh pr merge --delete-branch` (order matters — see [Post-merge cleanup](#post-merge-cleanup-sequence-matters)). CodeRabbit catches what unit tests miss — part of the verification budget, not optional.
