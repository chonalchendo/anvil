---
name: capturing-inbox
description: "Use when the user wants to dump a thought, idea, or half-formed observation into the inbox without acting on it. Triggers: 'capture this…', 'remind me to…', 'thought:', 'idea:', 'park this'. Prefer over writing-issue when in doubt."
---

# Capturing Inbox

Capture is the load-bearing habit of the whole methodology. If capture has friction, the user stops capturing, and every later skill starves. Your single job is to turn a thought into a file and get out of the way. You are a stenographer, not an editor.

## Shape test

**If you can name an acceptance criterion in one breath, it's an issue, not an inbox.** Capture is for fleeting / fuzzy ideas not yet sharp enough to write an AC. Shape, not domain: a fully-shaped thought about anvil itself still routes to `anvil:writing-issue`; a half-formed thought about anything routes here.

Wrong-choice example: user pastes a problem statement, acceptance bullets, and a milestone hint. That's an issue — hand off to `anvil:writing-issue` rather than capturing and forcing a later promote round-trip.

## Iron Law

**NO VALIDATION DURING CAPTURE.** Do not critique, classify, pressure-test, ask "are you sure," suggest scope, propose tags, or speculate about feasibility. `writing-issue` exists for all of that. From the user's side, capture is write-only.

## Phase 1 — Write the inbox entry

1. Take the thought as given. If the message starts with a trigger phrase ("capture this:", "thought:", "remind me to…", "for the inbox:"), strip the trigger and keep the rest verbatim. Otherwise use the message as-is.
2. Run:
   ```bash
   anvil create inbox --title "<short title from user's thought>" --json
   ```
   Capture `id` and `path` from the JSON output.
3. Direct-edit the body section of `path`, appending the user's thought verbatim under the frontmatter. Preserve voice, hedges, typos that aren't obvious slips, and incompleteness. Do not rewrite, expand, summarize, or "clean up."

## Phase 2 — Acknowledge and stop

Reply with one short line: `Captured → <path>`. Nothing else. Do not echo the thought back. Do not propose links, projects, milestones, or next actions. Do not ask follow-up questions about the thought itself.

## Multi-item dumps

If the user pastes something that is plainly several distinct thoughts — separated by blank lines, bullets, "also," or "and another thing" — ask **exactly once**: *"One item or split into N?"* Default to a single file if the answer is ambiguous or absent. Never try to auto-segment flowing prose; the cost of a wrong split is higher than the cost of one slightly-overstuffed inbox item, because brainstorming can split later but cannot reconstitute lost grouping.

## When the user signals more than capture

If a capture message also contains an invitation to engage ("capture this: refactor auth — actually, let's think about it"), do the capture first, then offer **once** to hand off to `anvil:writing-issue` on the captured inbox id. Never silently escalate; the inbox file is the durable artifact and must exist either way.

If the user is clearly already brainstorming — asking questions, weighing tradeoffs, requesting your opinion — say so and suggest switching skills rather than forcing the thought through capture as a formality.

## Out of scope

- No tagging, linking, folder choice, or project inference.
- No reading of other inbox items, no dedup, no merge, no cross-reference.
- No promotion to brainstorm / issue / plan / build.
- No editing or deletion of prior captures.
- No frontmatter authoring beyond what the CLI generates.

Capture is write-only and append-only. Reading happens later, in another skill, with more context than you have now.
