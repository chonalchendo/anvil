# The three levels (after Reis, Mixed Model Arts)

| Level | Question | Contains | Leaves out | Artefact |
|---|---|---|---|---|
| Conceptual | What does the system need to know? | Nouns with one-sentence definitions; verbs with cardinality and optionality; processes; scope on one page | Attributes, keys, types | One `erDiagram` per domain plus an overview |
| Logical | How is the data organised? | Attributes, keys, relationships with keys, class, grain, time axes, history, owner, intent, action | Technology, materialisation | Entity tables per domain; `erDiagram` with keys; target rows |
| Physical | How is it built in this technology? | Layer, name, types, materialisation, partitioning, tests, access class, retention, sizing, lineage, metadata | Nothing: precise and unambiguous | Table list per domain |

## Conceptual

- One noun, one concept. A customer is distinct from a lead.
- Relationships are a contract: is it optional? one or many?
- Scope check: the user looks at one page and says "yes, that is the domain" or "what about X?".

## Logical

- Normalise as a tool, not a religion.
- Use a single-column key; keep natural keys as indexed attributes (hybrid).
- One mapping table per external identifier family, effective-dated.

## Physical

- Names are business-friendly words after the project's prefix; no cryptic abbreviations.
- Comments say what, why and how to use.
- Relationships are declared so humans and agents can traverse them.
- Large tables are incremental on their time column.

## Anti-patterns (Reis)

- God entity: one entity models everything.
- Phantom entity: an entity with no clear definition.
- Split entity: one concept spread over several entities without reason.
- Temporal confusion: what an entity is mixed up with what it was.
