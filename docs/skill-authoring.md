---
title: "Anvil: skill authoring conventions"
project: anvil
created: 2026-04-27
updated: 2026-04-27
related: "[[system-design.anvil]]"
---

# Anvil: Skill Authoring Conventions

Authoring rules for Anvil's methodology skills and user-authored vault skills. Supports [`system-design.md`](system-design.md): architectural shape lives there, conventions here. CI enforces the mechanical rules; the rationale below is the design guidance behind them.

A SKILL.md is a **trigger contract** (description) + **deferred reference** (body). Different design constraints, failure modes, tests. Description's job: discrimination — fire on the right contexts, not adjacent ones. Body's job: workflow execution that survives one read.

## Two skill types

| Type | Body | Failure mode | Description style |
|---|---|---|---|
| **Workflow** | Multi-phase process the agent reads in full | Agent shortcuts past the workflow | **Triggers-only** — *when to use*, never *what it does* |
| **Knowledge** | Principles, patterns, gotchas the agent applies | Under-triggers — agent doesn't load when it should | **Pushy** — exhaustive positives + explicit negatives |

Workflow descriptions that summarize cause the agent to follow the description and skip the body. Knowledge descriptions under-trigger by default — they need expansive *when* coverage.

Anvil's methodology mapped (`ls anvil/skills/` or `anvil show skill <name>` for the live set):

| Skill | Side | Type |
|---|---|---|
| `extracting-skill-from-session` | meta | workflow |
| `researching` | meta | workflow |
| `writing-product-design` | design | workflow |
| `writing-system-design` | design | workflow |
| `writing-milestone` | design | workflow |
| `capturing-inbox` | execution | workflow |
| `writing-issue` | execution | workflow |
| `completing-issue` | execution | workflow |
| `dispatching-issue-fleet` | execution | workflow |
| `reviewing-pr` | execution | workflow |
| `responding-to-pr-review` | execution | workflow |
| `distilling-learning` | execution | workflow |
| `opening-thread` | session | workflow |
| `resuming-session` | session | workflow |
| `handing-off-session` | session | workflow |
| `self-testing` | meta | workflow |

Anvil's methodology is workflow-dominant. User vault skills are knowledge-dominant (library/tool/domain expertise).

## Reading a bundled skill

`anvil show skill <name>` prints the SKILL.md body for a bundled methodology skill, sourced from the binary's embedded bundle (same content `anvil install skills` deposits). Use this when smoke-testing a skill body change rather than grepping `anvil/skills/<name>/SKILL.md` directly. Skills are not vault artifacts, so the verb has no `--json`, `--body`, or incoming-links surface — output is the file, full stop.

## Description rules

- **≤250 chars practical.** Claude Code truncates listings at 250 (issue #881). 1024 is the validator hard limit but it's a trap. Front-load critical content; truncated tails still fire.
- **No XML angle brackets** (`<`/`>`). Validator-hard.
- **Third person.** "Use when..." or verb-first ("Extract text and tables..."). POV inconsistency hurts discovery.
- **Workflow: triggers-only.** Describe *when*, never *what*. "Use when executing plans - dispatches subagent per task with code review between tasks" caused the agent to run one review when the body specified two. Fix: strip workflow detail → "Use when executing implementation plans with independent tasks."
- **Knowledge: pushy and exhaustive.** Explicit positives (file types, library names, phrases) + explicit negatives ("Do NOT trigger when..."). Anthropic's `xlsx` skill enumerates adjacent formats by name.
- **At least one literal trigger phrase** (`or mentions "X"`) for explicit invocation. Pocock's tail-pattern. Cheap; adds invocation surface without semantic noise.
- **Negative triggers for siblings.** Every skill names plausibly-overlapping siblings. `creating-issue` ↔ `capturing-inbox`. `planning` ↔ `writing-product-design` (different abstraction). `refactoring` ↔ `systematic-debugging`. Mechanical, high-leverage.

## Body rules

- **≤200 lines target, 500 hard cap** (`wc -l` in CI). >200: extract `references/`. >500: split.
- **One Iron Law per skill, max.** Workflow skills get one ALL-CAPS anchor (`NO FIXES WITHOUT ROOT CAUSE INVESTIGATION FIRST`); rationale-prose for everything else. ALL-CAPS proliferation produces brittle compliance (Anthropic yellow-flag). Knowledge skills usually have no Iron Law — principles, not procedures.
- **Workflow: imperative checklist.** Numbered steps in order, validation gates between, rollback on failure. Anchors the agent's TodoWrite list.
- **Knowledge: reference-with-principles.** Philosophy → patterns → gotchas/antipatterns. Heavy material → `references/`.
- **User gates: own paragraph.** Verbatim quote-template + "Wait for the user's response" terminator. Visually distinct from prose so the agent doesn't blow past.
- **Handoffs: `**REQUIRED SUB-SKILL:** Use skill-name`.** Reference Anvil skills by bare name — they register flat, so an `anvil:` prefix resolves to `Unknown skill`. Prefix only skills from another installed plugin (`superpowers:<name>`). Never `@filename` (force-loads, burns context). Plain-language references resolve wrong intermittently (issue #1002).
- **On-demand body: `**REQUIRED REFERENCE:** Use skills/<name>/references/<mode>.md`.** Points at a `references/` file in the same skill directory. The agent loads it when that mode/kind is needed; the always-on SKILL.md stays lean. Use when a skill branches by kind (e.g. bug vs feature) or mode (e.g. fast vs deep) and each branch carries enough content to push the body over the 200-line target. Differs from `REQUIRED SUB-SKILL` in that it loads a file, not a skill — no trigger contract, no frontmatter, no registration; just a markdown body the agent reads inline.
- **Code-fence path/command examples.** Literal-text recipes have caused Claude Code to inject malformed Write calls (issue #1042). Fences + "the agent will..." framing prevent this.
- **Decision skills carry a legibility beat.** A skill that recommends a direction to the human (issue/design/build decisions) frames forks legibly *by default*, so the operator can steer without prompting for it. Two beats, expressed as principles inside existing phases — not new sections or templates: **fork-framing** (at a genuine fork: name the options, surface the rejected alternative *and why it fails*, give the discriminating fact, then recommend one direction — don't menu) and, where work lands, a **completion summary** (*what was done / why this shape / what to watch*). Legible ≡ brief: fire only at real forks, never manufacture tension, never license a longer response.

## Frontmatter rules

- **Allow-listed top-level keys**: `name`, `description`, `license`, `allowed-tools`, `metadata`, `compatibility`. Anything else fails import.
- **`name`**: kebab-case, `^[a-z0-9-]+$`, ≤64 chars, no consecutive/leading/trailing hyphens. Reserved: `claude`, `anthropic`.
- **`description`**: ≤1024 chars (validator), ≤250 (practical, see above). No XML brackets.
- **`compatibility`**: agent-CLI targets. Anvil's methodology: `Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard.`
- **`metadata`**: Anvil-specific fields (`vault_type`, `authored_via`, `confidence`, `status`, `source_learnings`). See [`vault-schemas.md`](vault-schemas.md).

## Skills are hypotheses, validated by use

A skill is a hypothesis about a recurring pattern, packaged for reuse. Source can be anything — successful session, learnings, research, a colleague. Packaging is constant; lifecycle produces value.

Three authoring paths, three meta-skills:

1. **`extracting-skill-from-session`** — crystallize a working in-context workflow. You did the activity, iterated, want to capture what worked. Best for workflow skills (especially Anvil's methodology). Strips session noise, identifies the load-bearing pattern, produces SKILL.md via `writing-skills`.

2. **`bootstrapping-knowledge-skill`** *(future)* — first-draft a knowledge skill on a domain you don't yet know. Calls `researching` (general-purpose research skill) as a sub-skill to gather best practices, then synthesises a draft body. Best for bootstrapping knowledge skills on new domains. The research half is general-purpose: `researching` also serves design / planning / inbox-promotion flows that need external facts; it is not skill-authoring-specific.

3. **`synthesizing-knowledge-skill`** — refresh a knowledge skill from accumulated learnings. Captured learnings in `~/anvil-vault/20-learnings/`; want the skill updated. Diffs new learnings against the existing skill, proposes updates.

All three invoke `writing-skills` as sub-skill for formatting. Provenance differs by path; reflected in skill metadata.

**Honest distinction**: research-derived skills are less reliable than experience-derived. Sources can be wrong, stale, or contextually off. Lifecycle that respects this:

1. `bootstrapping-knowledge-skill` (which calls `researching`) produces a draft knowledge skill at `confidence: medium`, `status: from-research-only`.
2. Skill auto-fires on real work; helps where it can, fails where it has gaps.
3. Each gap → learning in the vault.
4. Periodic `synthesizing-knowledge-skill` refresh incorporates new learnings.
5. Eventually `confidence: high`, `status: experience-validated`. Research bootstrapped; experience refined.

`~/anvil-vault/40-skills/` accumulates these. Anvil methodology skills are stable and small; user knowledge skills grow. Six months of sqlmesh + Anvil → your `sqlmesh-best-practices` is better than anyone else's: *your* learnings, *your* problems, *your* fixes.

**Rippable by design.** Every skill should die well. When a model substrate, a host CLI feature, or a better skill makes one redundant, removing it should leave no scars: no other skill should depend on its specific shape, no artifact should encode its quirks, no workflow should assume it stays. Authoring against this constraint keeps the skill set honest about lifespan — skills earn their place per session, not per author-effort. The retirement signal is the same as the auto-fire signal inverted: a skill the agent stops needing is a skill that has done its job.

## Authoring workflow

Per Anthropic's skill-building guide ("iterate on a single task before expanding"):

1. **Identify a real recurring activity.** Not "someone might want this" — "I do this every week."
2. **Do it for real with Claude Code.** Iterate until it works.
3. **Run the meta-skill** (`extracting-skill-from-session` for workflows, `bootstrapping-knowledge-skill` for knowledge bootstraps, `synthesizing-knowledge-skill` for refreshes).
4. **Meta-skill produces SKILL.md** via `writing-skills`.
5. **Test before shipping.** 10-20 trigger-eval queries (mix of should-fire / should-NOT-fire), 3 runs each. Aim ≥90% on relevant, ≤10% on unrelated. Trigger-eval harness deferred to v0.2+; in v0.1 the authoring agent self-checks the trigger contract during phase 6 of `extracting-skill-from-session`.
6. **Iterate on real use.** Skills are living. Each gap → learning; refresh periodically.

## Diagrams as deliverable content

When a skill's output benefits from visual structure (architecture, dependencies, sequences, milestone roadmaps): mermaid blocks in the produced markdown. Diagram = content, not decoration. Lives in the artifact.

Why mermaid: text-based (clean commits, PR-diffable), renders inline in Obsidian and GitHub, no external tooling. Skill body walks the user through; agent generates source, user reviews.

Where:

- **`writing-system-design`**: context (boundaries), component (internal pieces), data flow (critical paths). Core deliverables, not afterthoughts.
- **`writing-product-design`**: gantt for milestone roadmap when timing matters.
- **`defining-milestone`**: graph for non-trivial predecessor/successor webs.
- **`planning`**: wave structure (task dependencies) as graph. More readable than YAML.

Where not: conversational execution skills (`human-review`, `capturing-learnings`, `re-entry`). Dialogue, not diagrams.

Borrowed from Superpowers' `brainstorming` (decision-tree visual for ideation). There: navigation aid. Here: part of the deliverable. Same tool, different purpose.

## CI validation

`.github/check-skills.sh` runs in CI and the pre-commit hook (shared script, so the two can't drift) over `anvil/skills/*/SKILL.md`:

- **File length** — fail over 500 lines, warn over 200 (whole-file `wc -l`, frontmatter included). Over target → extract to `references/`.
- **Description length** — fail over 1024 chars, warn over 250 (Claude Code truncates skill listings at 250).
- **Schema alignment** — `anvil validate skill` scans each authoring skill's SKILL.md for prescriptive frontmatter directives (`Capture frontmatter \`field\``, etc.) and fails with `skill_schema_drift` if a directed field is absent from its target type's schema. The check only sees directives written with a recognized prefix (`Capture`/`Populate`/`Add to frontmatter \`field\``, `frontmatter field \`field\``); phrase frontmatter directives that way so they're scanned. Run after editing a writing-* skill or after changing a schema's `properties`.

Left to authoring discipline (and Claude Code's own import validation), not gated here: the frontmatter key allow-list + name regex, the no-XML-brackets rule above (shipped descriptions use `<id>`/`<source>` placeholders that import accepts, so it is not mechanically gated), one-ALL-CAPS-per-body, and sibling negative-triggers.

A co-load library smoke test — load all methodology skills against a diverse prompt set, check for trigger conflicts and context-budget overruns — is deferred to v0.2+: individual skills can validate cleanly yet interact badly when co-loaded, so v0.1 relies on hand-vetted bundling.
