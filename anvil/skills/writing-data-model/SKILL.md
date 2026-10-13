---
name: writing-data-model
description: "Use when designing a project's data model: entities, relationships, keys, grain, time axes and tables, conceptual to logical to physical. Requires a system design. Not for interfaces (writing-component-design) or one table's code (completing-issue)."
license: MIT
allowed-tools: [Bash, Read, Edit, Write, Agent]
compatibility: "Works with Claude Code 2.0+ and Codex 0.121+ via SKILL.md standard"
metadata:
  vault_id: writing-data-model
  vault_type: skill
  skill_type: workflow
  side: design
  created: 2026-10-12
  updated: 2026-10-12
  tags: [type/skill, activity/data-model]
  diataxis: how-to
  authored_via: hand-authored
  confidence: low
  status: from-first-use
---

# Writing Data Model

A workflow for designing a project's data model: the map of every entity the system holds, how entities relate, and how each becomes a table. It runs three levels in order, after Joe Reis (*Mixed Model Arts*): **conceptual** (the town plan), **logical** (the blueprint), **physical** (the system). It records the **target state only**. Current tables are evidence of failure modes, never the starting point.

The data model sits between the system design and the component designs. Components own entities; the data model makes them one coherent landscape.

## Iron Law

**NO TABLE WITHOUT A DEFINED NOUN, A DECLARED GRAIN AND CHOSEN TIME AXES.**

## When to use

- A system design exists, and the project stores data that more than one component reads.
- The project crosses modelling camps: relational, analytics, application, ML/AI, knowledge.
- User says "data model", "entities and relationships", "ERD", "grain", "bitemporal", "what tables do we need".

## When not to use

- No system design yet → `writing-system-design` first.
- One component's interface → `writing-component-design`.
- One table's code or migration → `completing-issue`.
- One standing modelling choice → a decision (`anvil create decision`).

## Phase 0: Frame

1. Read the system design and every active component design: `anvil show system-design <project> --body`, `anvil list component-design --project <project>`.
2. Build an entity inventory from the designs, not from code. Dispatch a subagent to list, per stored or passed thing: owner, readers, grain, keys, time axes, mutability, citation. It also lists cross-boundary identifiers and conflicts (one concept two names, one name two meanings, no owner, never minted).
3. Load the modelling checklist. **REQUIRED REFERENCE:** Use skills/writing-data-model/references/checklist.md
4. Cut the landscape into domains: boundaries of meaning, each with one owner subsystem.

**Gate:** present the domains and the inventory counts.

> Domains: <list>. Inventory: <n> entities, <n> identifiers, <n> conflicts. Confirm the domain cut before conceptual work?

Wait for the user's response.

## Phase 1: Conceptual, one domain at a time

**REQUIRED REFERENCE:** Use skills/writing-data-model/references/levels.md

Diagram boxes and marks: skills/writing-data-model/references/diagrams.md.

For each domain, reference domains first (everything joins to them):

1. **Nouns.** Define each entity in one sentence. Merge duplicates, split homonyms, cut phantoms.
2. **Verbs.** State each relationship with cardinality and optionality ("a listing has one security; a security has one or more listings").
3. **Processes.** Walk each business process that touches the domain: trigger, ordered steps, change, result.
4. **Instances.** Walk two or three real-world cases with real values through the nouns and verbs. A case that does not fit exposes a missing noun or verb.
5. **Anti-patterns.** Check for a god entity, a phantom entity, a split entity and temporal confusion.
6. **Diagram.** Entities and relationships only, no attributes, with cardinality at each end. Keep a mermaid `erDiagram` as the text source. Render every reviewed view with the `diagram-design` ER / data model type: the same two-section box as the logical level, with the noun in the header and a one- or two-line gloss (not attributes) in the body. Keep at most 8 entities per view, so split a large domain into views (for example identity, place and time, grouping). Draw another domain's noun dashed with its domain tag; tag a noun drawn in full in another view of the same domain "VIEW n".
7. **Validate against reference models.** Start a researcher when the domain starts, in parallel with the draft. It compares the draft with the established models for the domain (standards, vendor data models, literature) and reports missing nouns, wrong cardinalities and contested merges or splits, with citations. Fold surviving findings in before the gate. Standards are evidence, not authority: a finding changes the model only if it serves the project.

**Gate:** the user confirms the domain diagram reads as the domain, with the validation findings folded in. Then the next domain.

**Overview.** After the last domain, draw one diagram at domain grain:
- one box per domain, with its noun count and two or three key nouns;
- one line per borrowing, labelled with the borrowed nouns, never with cardinality;
- the cross-cutting domains (provenance, metadata, operations) as a band below, with a note instead of a line to every domain.

Before you draw, check that every borrowed noun has exactly one owner. Count nouns from the domain tables, not the diagrams.

## Phase 2: Logical

**REQUIRED REFERENCE:** Use skills/writing-data-model/references/time-axes.md

1. **Cross-cutting rules first**, each ratified with rows behind it: identity and key minting, the time vocabulary, the as-of read, identifier kinds, entity classes, history policy, schema change, semantic metadata.
2. **Per entity:** attributes; keys; relationships with their keys and cardinality; class; grain; time axes chosen by the three questions; history policy; owner; intent (the questions it answers); action (valid operations and state transitions).
3. **Walk rows.** Show target rows with values for each domain's core entities, and one as-of read.
4. Resolve the inventory's conflicts for the domain; list any left open.
5. **Validate time and attributes.** A narrower researcher pass checks how reference systems hold the same entities over time (point-in-time history, revisions, restatements). Fold surviving findings in before the gate.
6. **Draw the logical ERD** with diagram-design's ER / data model type, the same type as the conceptual level (not the database-schema type, which is physical). Name entities in PascalCase; attributes stay snake_case. Use two-section boxes: the class tag and name, then fields with `#` for a key, `→` for a reference, and one muted time line naming the axes. Keep at most 8 entities per view, and show every entity in its home view. Draw a polymorphic reference as one dashed box listing its targets. Leave a redundant reference (one an audit derives) as a field only, with no line.

**Gate:** per domain, the user confirms the entity table, the rows and the ERD.

## Phase 3: Physical

Physical is where techniques mix (Reis, *Mixed Model Arts*: "same meaning, different physics"; each layer needs its own approach). One logical entity can be built once per layer.

0. **Layers first.** State the layer list, each layer's role, what it may hold and what it may read. Draw it with the `diagram-design` medallion type: one tier per layer, with its writer, tool, format and an example table. Then place every logical entity: which layer or layers implement it, and as which tables.
1. **Home.** Choose each entity's store from the system design: an analytical table (with its layer), an operational database, an object store, a code declaration, or an external system. A code declaration a run must cite also gets a recorded version row.
2. **Technique per layer.** Choose the modelling style per layer and camp with a reason: dimensional for analytics, point-in-time tables for ML, append-only event logs with current-state views for application records, catalog metadata for knowledge.
3. **Naming.** Use the project's table-name convention. Define one prefix per class if the project uses prefixes.
4. **Per table:** layer, name, materialisation (incremental on a named time column; full only with a reason; view only where the project allows), partitioning or clustering, types, tests and audits, access class, retention, lineage from source to consumer.
5. **Sizing.** Expected rows, bytes and growth per table, from measurement where data exists. Size decides materialisation, not habit.
6. **Metadata.** Every table and column carries technical, business and semantic metadata.
7. **Schema diagrams.** Draw each layer's tables per domain with the `diagram-design` database-schema type: real types, constraint chips and column-to-column references, grouped in a container per layer. Keep to its budget of 5 tables per view. Where the engine enforces no constraint, label each reference edge with the audit that enforces it, not a referential action.

**Gate:** the user confirms the layer map once; then, per domain, the table list, its settings and the schema diagrams.

## Phase 4: Integrate

1. Record each standing rule as a decision; link it.
2. Link each component design to the entities it owns.
3. Update the system design where the data model changed a boundary.
4. Run `anvil validate`. List every conflict still open as an open question.

**Completion summary:** what was modelled, why this shape, what to watch.

## Common mistakes

- **Starting from today's tables.** They encode the failures the model must fix. Use them only as evidence and for sizing.
- **One time rule for everything.** Choose axes per entity; tritemporal where decisions and corrections both matter, a single timestamp where they do not.
- **Hand-kept lists of outside facts.** A rare or textual source still gets a connector; a curated seed hides its timing and cannot rebuild. Seed only your own declarations and corrections.
- **Physical before logical.** A table with no logical entity has no owner, grain or meaning.
- **Attributes on the conceptual diagram.** The town plan shows nouns and verbs only.
- **Wrong diagram type for the level.** Conceptual and logical use the ER / data model type (box-to-box lines with cardinality). Physical uses the medallion type for layers and the database-schema type for tables (column-to-column lines). Never mix entity names and table names in one diagram.
- **Skipping instances.** A diagram that no real case has walked through hides missing nouns.

## Prose style

Write all prose (artifacts, reports, replies) about 80% of the way to ASD-STE100 Simplified Technical English. Keep domain terms; skip the approved-word dictionary.

- Short sentences: 20 words at most for an instruction, 25 for a description.
- Active voice. One instruction per sentence. Conclusion first.
- One term per concept; reuse it verbatim.
- No filler, no hedging, no restating what the reader already has.
