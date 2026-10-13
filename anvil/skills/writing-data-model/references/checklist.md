# Modelling checklist (decide each explicitly)

Identity: stable internal key per entity; key lifecycle on merge, split, rename, reuse; mapping table per external identifier family, effective-dated; who mints each cross-boundary identifier; one key rule per kind.

Grain: declared grain per table; join and roll-up rules between grains.

Time: axes per entity (time-axes.md); one as-of read contract; restatements append, never overwrite; late and backdated data policy.

History: policy per entity (append-only, effective-dated state, snapshot, immutable version); which layers are immutable and which rebuild; can a past run be rebuilt from pinned inputs.

Semantics: one written definition per term; conformed dimensions with one owner; layer roles; technique per layer chosen with a reason; conceptual model separate from schema; which semi-structured fields become columns.

ML and AI: feature (entity, grain, event and knowledge time, source); label (horizon, outcome times, missing outcomes); training-set row (entity, as-of, feature and label versions); prediction (run, entity, time, input reference, output; outcome joined later); run lineage.

Events and state: which records are events (append-only) and which are state; event-sourced or state-based for money and orders, decided early.

Ownership and change: owner per entity; schema change rules (additive, versioned breaking change, deprecation); naming (one term per concept; time and key suffixes); synthetic data marked and never mixed with source facts.

Semantic layer: technical, business and semantic metadata on every table and column; intent (questions each entity answers); action (valid operations, state transitions, policies).
