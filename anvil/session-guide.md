# Using anvil

An anvil vault backs this session. It holds the project's designs, milestones, issues, conventions and learnings. Read and change it through the `anvil` CLI, not by editing vault files.

## Find context

- `anvil where` — the current vault and project.
- `anvil list <type>`, then `anvil show <type> <id> --body`. Types include product-design, system-design, milestone, issue, contract, convention, decision, learning.
- `anvil hydrate <issue-id> --tldr` — all context that governs one issue, in one call.
- `anvil --help` and `anvil <verb> --help` — every verb and flag.

## Skills before CLI

When an anvil skill covers the activity, fire the skill, not the raw CLI. Examples: `writing-issue` creates an issue, `completing-issue` implements one, `capturing-inbox` parks a passing thought, `distilling-learning` records a finding. Mechanical verbs (`list`, `show`, `hydrate`, `link`, `where`, `validate`) are fine to call directly.

## Work an issue

- Pick from `anvil list issue --ready --json`.
- Claim it: `anvil transition issue <id> in-progress --owner <name>`.
- Search before you create: `anvil list <type>` and `anvil link --to <id>`.
- Resolve only after the human merges the change.

