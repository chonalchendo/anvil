# Diagram conventions

Draw with the `diagram-design` plugin. Pick the format by level: `ER / data model` for conceptual and logical, `medallion` for the physical layer map, `database-schema` for physical tables. Keep a mermaid `erDiagram` as the text source for ER views.

## The box (ER / data model)

Two sections.

- Header: class tag and PascalCase entity name.
- Body: one field per line, snake_case.

| Mark | Meaning |
|---|---|
| `#` | Key field. |
| `→` | Reference to another entity. |
| `#→` | Key part that is also a reference (for example a composite key that includes a parent key). |
| muted `time:` line | The time axes of the entity, for example `time: valid, knowledge, processing`. |

The conceptual box keeps the header and a one- or two-line gloss. It has no fields.

## Dashed boxes

Draw a dashed box for an entity that is not fully drawn here:

- an entity shown in full in another view of the same domain (tag it `VIEW n`);
- an entity owned by another domain (tag it with the domain name);
- a deferred entity that the model names but does not yet define.

A polymorphic reference is one dashed box that lists its targets.

## Budgets and rules

- At most 8 entities per ER view; at most 5 tables per database-schema view.
- Show every entity in full in exactly one home view.
- Leave a redundant reference (one an audit derives) as a field with no line.
- Never mix entity names and table names in one diagram.
