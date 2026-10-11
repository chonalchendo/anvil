# Team mode

Use this when the project's `CLAUDE.md` or `AGENTS.md` says `Merge approval: team`. A teammate approves the milestone once, as one GitHub stack. Self mode keeps "Never stack" and `--land-pr`.

## Stack the issues

1. Put the issues in landing order. One line, no forks.
2. Cut the first layer from the default branch: `anvil transition issue <id> in-progress --owner <name> --cut-worktree`.
3. Cut each later layer from the top branch of the stack so far, after the layer below has a PR:
   `anvil transition issue <id> in-progress --owner <name> --cut-worktree --base <stack-top-branch>`.
4. Fill `Base: <stack-top-branch>` in each worker dispatch for layers above the first. The worker opens its PR with `gh pr create --base <branch>`.
5. Dispatch one layer at a time. A layer needs its predecessor's branch.

## Link the PRs

After the PRs exist, link them into one GitHub stack, bottom to top. Never run `gh stack` against a repo you do not own.

```bash
gh stack link <pr-bottom> <pr-next> ... <pr-top>
```

`gh stack link` keeps no local state and corrects PR bases to the chain. Add `--base <default>` when the default branch is not the trunk. After a later layer opens, run the same command again with all PRs; it only adds.

## Per layer

Run the Phase 4 verdict gate and Phase 5 review at each layer's head, as in self mode. Land nothing yet.

## Stack complete

Stop with "stack complete" when all of these hold:

- Every layer has a `pass` verdict at its head.
- Every layer has an agent review round at its head, with no open blocker, high or actionable medium finding.
- CI is green on every layer.
- `anvil milestone status <milestone-id> --at <top-branch> --json` is green.

Report the stack: the PR list bottom to top, each verdict and review result. Stop. The human sends the stack to the teammate. Do not wait for approval.

## Teammate change request

1. Dispatch `anvil-pr-responder` into layer k's worktree with the request.
2. Restack the layers above: `gh stack rebase --upstack` from layer k, then `gh stack sync` to push. Resolve conflicts with `gh stack rebase --continue`.
3. Re-run the verdict gate on each layer above k. A review whose layer patch did not change stays fresh. Re-fire `reviewing-pr` only on a layer whose patch changed.
4. Report "stack complete" again.

## Land

After the teammate approves, the human approves the land. Run from the parent checkout:

```bash
anvil transition issue <top-issue> resolved --land-stack <top-pr>
```

Branch protection enforces the teammate's approvals. Never check `reviewDecision` yourself. Never run `gh pr merge`. After the land, pull the base branch (`git pull --ff-only`) and continue at Phase 7.
