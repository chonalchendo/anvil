<!-- Keep this PR tight. Brevity > completeness — a reviewer should grasp it in 60s.
     Cut any section that doesn't apply rather than padding it.
     Write about 80% of the way to ASD-STE100 Simplified Technical English: at most
     20 words per instruction sentence and 25 per description, active voice, one
     instruction per sentence, conclusion first. Keep domain terms.
     Full rules: `anvil show convention convention.prose --body`. -->

Resolves <anvil-id>

## What changed
<!-- The change surface, in 1-3 bullets. Not the how. -->

## Why
<!-- The problem / AC this serves. Link the issue's goal. -->

## How
<!-- Key implementation decisions a reviewer needs to follow the diff. Skip the obvious. -->

## Test Plan
<!-- REQUIRED: live smoke captured verbatim — the real `anvil <verb>` driven through the
     installed binary against a real vault, plus the output it produced. Green `go test`
     does NOT count (smoke-test gate, docs/worktree-workflow.md). If genuinely N/A
     (docs-only / skill-body-only), say so and how you verified instead. -->

## Risk
<!-- One line: anything irreversible? (vault write, schema break, installer/hook change) Else "none". -->
