# Behavioral Guardrails

How to approach work in Anvil — non-negotiable behaviors that apply before and during any code or design change.

## Reading Discipline

Reading source is the single highest per-token cost in this repo. Before any `Read`:

- **Grep for the symbol first.** `grep -n "FuncName\|ConstName" path/` returns line ranges; `Read` with `offset`/`limit` around the match (~60-80 lines of surrounding context usually suffices). Bug-fix work rarely needs the whole file.
- **Whole-file reads earn their place.** Justified when (a) authoring a new file, (b) the file is small (<150 lines), or (c) grep matches scatter across most of the file. Otherwise narrow first.
- **Don't re-read files you just wrote.** `Edit` / `Write` fail loudly on mismatch; the harness tracks post-edit state.
- **Scan tool output, don't echo it.** When `go test`, `anvil list`, or similar produces long output, extract the headline (pass/fail count, first failure name) — don't paste the full dump into subsequent reasoning.

The failure mode this prevents: reading a 600-line file end-to-end when the bug lives in 60 of those lines. The cost compounds across a session.

## Think Before Coding

Don't assume. Don't hide confusion. Surface tradeoffs.

Before implementing:

- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them — don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

**YOU MUST stop and ask the user, not guess, when:**

- Adding a new dependency. Even a small one.
- Choosing between two genuinely different architectural approaches.
- Refactoring code outside the scope of the current task.
- Adding a new artifact type, skill, schema, or top-level directory.
- The design doc is silent or ambiguous on a structural question.
- Implementation is taking longer than expected and you're considering shortcuts.

Don't stop and ask for: trivial naming choices, where to put a clearly-bounded helper, formatting decisions covered by `golangci-lint` config.

## Surgical Changes

Touch only what you must. Clean up only your own mess.

When editing existing code:

- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it — don't delete it.

When your changes create orphans:

- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

**The test: every changed line should trace directly to the user's request.**

## Goal-Driven Execution

Define success criteria. Loop until verified.

Transform tasks into verifiable goals:

- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:

    1. [Step] → verify: [check]
    2. [Step] → verify: [check]
    3. [Step] → verify: [check]

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## Vault Hygiene

**Obsidian wikilink stubs.** Clicking an unresolved `[[issue.foo.bar]]` link in Obsidian stamps a 0-byte `issue.foo.bar.md` at the vault root. `anvil reindex` flags these on stderr (`WARN: 0-byte stub at vault root: ...`); `anvil reindex --prune-stubs` deletes the 0-byte ones. Non-zero files at the root with type-prefixed names are reported but never auto-deleted — move them into the canonical `<NN>-<type>/` dir or remove by hand.

## End-of-Session Token Reflection

Before closing a dogfood session: rough total, top 2–3 token sinks (avoidable reads, redundant searches, oversized tool output), and any harness/CLI/skill change that would've cut them. A session with no token-side observation is itself a finding.
