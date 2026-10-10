# Writing Product Design — Phase procedure

Loaded on demand from `writing-product-design/SKILL.md`. Each phase has an explicit user gate — gates are the load-bearing part. Each is an iteration loop; expect 1–2 reframes per phase.

### Phase 1 — Frame

Confirm scope: project slug, what counts as the product (one or several), and that it saves through `anvil create product-design`.

Say up front: workers do not load this doc. Write it for the human. Do not put worker instructions in it. The doc is explanation: why and what, never how-to steps.

**Gate:** scope, slug, save route confirmed.

### Phase 2 — Problem & users

Elicit from conversation; there is no source doc; a re-author reads only the PD it replaces.

Draft body sections:
- **Why it matters** — leads with the one-sentence problem (2–4 short paragraphs).
- **Who it's for** — 1–3 user descriptions with implicit negative space (who it's *not* for; 2–3 short paragraphs).

**Voice check before drafting.** Sample the project's voice if any prose exists; audit drafts for AI-generic hedging, abstract framing ("In today's landscape..."), corporate-speak. Direct, declarative, specific. Concrete details, not analogies.

**Gate:** distillation reflects intent. Reframes expected.

### Phase 3 — What we're building (LOAD-BEARING)

Draft the **What we're building** body section:
- Lead with one-line product shape ("X is a Y for Z, packaged as…").
- Follow with user-facing convictions — what the user *feels*, not *how it's built*.
- End with one-line user-touch summary ("the user does X at most; everything else is Y").

**Keep *what* separate from *how*.** Implementation strategy, packaging, subprocess choices belong in `system-design.md`. Strip them.

**Gate (load-bearing):** premise confirmed. Phase 5 derives milestones from this — often takes 2–3 passes.

### Phase 3.5 — Approach (fat-marker sketch)

Draft the **Approach** body section: a fat-marker sketch of the solution shape. Gives Phase 5 something to derive from without breaching what-vs-how.

> **Broad strokes only — no architecture, no tech choices, no APIs.** If a bullet names a database, framework, library, or API, push it to `system-design.md`.

3–7 bullets at whiteboard altitude. Right altitude examples:

- "User runs one command per phase; everything else is automated."
- "Skills auto-load by file presence — no registry."
- "Telemetry is opt-in, local-only, queryable from the CLI."

Wrong altitude (push to system-design): "Use SQLite via modernc.org driver", "Cobra command tree with three subcommands".

**Gate:** altitude is right — too detailed → strip; too vague → expand.

### Phase 4 — Goals and measures, constraints, out-of-scope (LOAD-BEARING)

All three are body sections (prose under headings); no frontmatter arrays.

**Past-pain → measures prompt (critical).** Ask explicitly: *what should success measurably not be? What failure modes from past tools do you want a measure guarding against?* Old-tool failure modes are the strongest source of concrete measures.

Draft body sections:

- **Goals and how we measure them** — 3–5 goals, each paired with a measure. A goal is outcome-shaped. A measure is checkable, quantitative or qualitative. For a qualitative measure, name how you will check it (informal survey, telemetry signal). Examples:
  - Goal "Users feel the tool is on their side"; measure "weekly informal check-in, 4 of 5 say yes".
  - Goal "Plans work first try"; measure "≥80% of plans auto-fire on first try".

- **Constraints & appetite** (Shape Up). Constraints are usually fixed-time; scope is the variable. Appetite is the explicit time box (`small-batch` 1–2 weeks / `big-batch` 4–6 weeks / explicit duration). 2–5 constraint bullets mixing capacity, deadline, dependency. "v0.1 ships in 6 weeks" is a constraint; "no UI" is out-of-scope.

- **What's deliberately out of scope** — 5–7 items shaped as `<topic> — <why this is out, not just "we won't do it">`. Negative space prevents scope creep; "why" turns a no into a no with reasons.

**Gate (load-bearing):** explicit sign-off on goals and measures, constraints, appetite, out-of-scope. **Time-box:** if any list doesn't land in two iterations, write `TODO: validate after two weeks of real use` placeholders.

### Phase 4.5 — Risks, rabbit holes, open questions

Draft the **Risks, rabbit holes, open questions** body section. Brief — single gate, ≤5 minutes. Placeholders allowed.

Prompt: What could derail this? What rabbit hole are you afraid of? What's still genuinely open?

3–7 bullets. Examples:
- "Subprocess streaming buffer overflow on long tool-result lines"
- "Companion-pack drift if Superpowers reshapes its skills"
- "Open: should skills be content-addressed or path-based?"

**Gate:** list confirmed. Naming a risk often reveals it's actually a constraint or out-of-scope item — accept the reframe.

### Phase 5 — Initial milestones

Draft the **Milestones** body section. It lists open candidates only. A candidate leaves the list when its milestone is done.

Each candidate is one top-level plain bullet (`<slug> — one-line summary`) with no `[[…]]` link, then two nested lines. A link to a milestone that does not exist yet makes the save fail with `unresolved_link`:

```markdown
- <slug> — one-line summary
  - Why now: what makes this the next thing to build
  - Components: the system-design components it touches (write `TBD` until the system design names them)
```

Linking: `writing-milestone` Phase 4 rewrites the bullet to the shaped form and adds the `related` edge.

**Deferral:** a redesign may not scope milestones until the system and component designs exist. Then write one sentence under `## Milestones`: "Milestones deferred until system and component designs exist." Add no placeholder candidates, and skip `writing-milestone`.

**REQUIRED SUB-SKILL:** `writing-milestone` (a.k.a. `defining-milestone`).

If unavailable, collect titles + summaries inline as plain bullets.

**Gate:** breakdown confirmed, or deferral stated.

### Phase 6 — Serialize & save

1. Write `## TL;DR` (5 lines or fewer) as the first body section. Bump `updated` to today.
2. Hand-check the frontmatter and body:
   - Required frontmatter: `type, title, description, created, status, project`.
   - Optional frontmatter: `updated, tags, aliases, related, external_links`.
   - **No other frontmatter fields** — schema is `additionalProperties: false`. If you wrote `goals:`, `risks:`, `milestones:`, `target_users:`, `revisions:` as frontmatter arrays, move them to body sections.
   - Body has the required sections in order (`anvil create product-design --show-template` prints the headings).
   - No worker instructions or pointers to repo docs anywhere in the body.
   - Each top-level `- ` bullet under Milestones carries `Why now:` and `Components:`. Bullets are plain text until their milestone exists.
3. Write the body (no frontmatter) to a temp file. Save with `anvil create product-design --project <slug> --title "…" --description "…" --body-file <file>`. Create validates frontmatter and wikilinks, and rolls back on failure. It does not check body sections. It writes `status: draft`, so then run `anvil transition product-design <slug> active`. Re-authoring an existing PD: add `--update`; it keeps status and `related`, so skip the transition.
4. Read it back with `anvil show product-design <slug> --body` for the cold read.

**Gate:** user reads the artifact cold. Capture the project's vision? Fix and re-show if not.
