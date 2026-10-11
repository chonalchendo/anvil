# Team mode

Use this when the project's `CLAUDE.md` or `AGENTS.md` says `Merge approval: team`. A teammate approves the milestone once, as one GitHub stack. The human then approves the land once for the whole stack. Self mode keeps "Never stack" and `--land-pr`.

## Stack the issues

1. Put the issues in landing order. One line, no forks.
2. Cut the first layer from the default branch: `anvil transition issue <id> in-progress --owner <name> --cut-worktree`.
3. Finish the layer below before you cut the next: PR open, Phase 4 verdict gate, Phase 5 review and every fix. A later fix on a lower layer leaves the layers above on a stale base.
4. Cut each later layer from the top branch of the stack so far:
   `anvil transition issue <id> in-progress --owner <name> --cut-worktree --base <stack-top-branch>`.
5. Fill `Base: <stack-top-branch>` in each worker dispatch for layers above the first. Dispatch one layer at a time.

## Link the PRs

Run `gh stack` in the project repo. After the PRs exist, track the stack locally once, bottom to top. `gh stack rebase` and `gh stack sync` exit 2 without it.

```bash
gh stack init --base <trunk> <branch-bottom> ... <branch-top>
gh stack link <pr-bottom> <pr-next> ... <pr-top>
```

`gh stack link` corrects PR bases to the chain. Add `--base <trunk>` when the trunk is not the default branch. After a later layer opens, add its branch to the stack (run `gh stack add <branch>` from the current top layer's worktree) and run `link` again with all PRs; it only adds.

## Per layer

Run the Phase 4 verdict gate and Phase 5 review at each layer's head, as in self mode. Land nothing yet.

## Stack complete

Stop with "stack complete" when all of these hold:

- Every layer has a `pass` verdict at its head.
- Every layer has an agent review round that is not stale (the land gate judges staleness by the layer patch), with no open blocker, high or actionable medium finding.
- CI is green on every layer.
- `anvil milestone status <milestone-id> --at <top-branch> --json` is green.

If the finish line is red at the stack top, return to Phase 1. Derive the next issue and cut it as a new top layer.

Report the stack: the PR list bottom to top, each verdict and review result. Stop. The human sends the stack to the teammate. Do not wait for approval.

## Teammate change request

1. Dispatch `anvil-pr-responder` into layer k's worktree with the request.
2. Restack the layers above: `gh stack rebase --upstack <branch-k>`. Resolve conflicts with `gh stack rebase --continue`. Then run `gh stack push` from the top layer's worktree. `gh stack sync` also pushes, but only its own rebase.
3. Re-run the verdict gate on layer k and on each layer above it. A review whose layer patch did not change stays fresh. Re-fire `reviewing-pr` only on a layer whose patch changed.
4. Report "stack complete" again.

## Land

Before the land, check that the base branch has not moved past the stack. If it has, run `gh stack rebase <branch-top>`, then `gh stack push` from the top layer's worktree, then repeat the per-layer gates. A push can dismiss the teammate's approvals when branch protection dismisses stale approvals. Tell the human, so the teammate approves again.

After the teammate approves, the human approves the land. Run from the parent checkout:

```bash
anvil transition issue <top-issue> resolved --land-stack <top-pr>
```

Branch protection enforces the teammate's approvals. Never check `reviewDecision` yourself. Never run `gh pr merge`. After the land, pull the base branch (`git pull --ff-only`) and continue at Phase 7.
