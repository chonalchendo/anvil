---
name: researching
description: "Use when researching a library, technique, or domain - standalone (\"research ducklake\", \"look into X best practices\") or as sub-skill before design / planning skills. NOT for clarifying intent (brainstorming) or recording a chosen path (decision-making)."
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
---

# Researching

External-fact gathering on a library, technique, or domain. Two invocation paths (standalone vs sub-skill), three depth modes (light / adversarial / heavy). The mode-specific procedure lives in `references/`; this file owns the framing, mode selection, and capture.

## Phase 1 — Scope

Branch by invocation path.

**Sub-skill mode** (a caller skill invoked this one with a question). Confirm and bound the question in a single round-trip with the caller, then proceed. Do not negotiate with the user.

**Standalone mode** (user invoked directly). Negotiate with the user to frame the question concretely and pin "good enough":

> What's the concrete question, and what does "good enough" look like — decision-grade comparison, or 60-second sniff test?
>
> Wait for the user's response.

## Phase 2 — Mode select

Default to **adversarial** unless the call is clearly low-stakes curiosity (use **light**) or a decision rides on the outcome (use **heavy**). State the chosen mode and one-line reasoning before loading the reference. The user can override with "quick research on X" / "challenge X" / "research X heavily".

**REQUIRED REFERENCE:** Use skills/researching/references/<mode>.md

The reference owns Gather / Challenge / Synthesise. Return here for Capture (standalone only).

## Phase 3 — Capture (standalone only)

Sub-skill mode skips Capture: synthesis is returned to the caller, which absorbs it into its own artifact.

For standalone runs, persist zero-or-more `learning` artifacts. Count is agent judgment by coherence — no forced N.

1. Discover existing tags so new captures reuse the established taxonomy:

   ```bash
   anvil tags list --type learning --json
   ```

2. Propose candidate learnings to the user. Each candidate: working title, core finding (1–2 sentences), source URLs, confidence (`low` / `medium` / `high`), suggested tags drawn from the existing list where possible.

3. User gate:

   > Here are the candidate learnings. Confirm, edit, or discard each. Anything else worth capturing that I missed?
   >
   > Wait for the user's response.

4. For each accepted candidate, create the artifact and attach tags / related links:

   ```bash
   anvil create learning --title "<title>" --body "<body>"
   anvil set learning <id> tags --add <tag> [--add <tag> ...]
   anvil set learning <id> related --add <wikilink> [--add <wikilink> ...]
   ```

   `related` points back to whatever the research informed (caller artifact, issue, plan); leave empty for pure curiosity.

## Boundaries

Sibling skills — name a different one if the user's intent is shaped differently:

- `anvil:brainstorming` — clarifying the user's own intent before external facts matter.
- `anvil:decision-making` — recording a chosen path with rationale, not gathering options.
- `anvil:exploration` — poking at the local codebase or installed capabilities, not external sources.

Composes-with (callers that invoke this as a sub-skill): `anvil:writing-product-design`, `anvil:writing-system-design`, `anvil:creating-issue`, `anvil:planning`.
