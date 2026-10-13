# Diagram conventions

Draw with the `diagram-design` plugin. Pick the format by level: `ER / data model` for conceptual and logical, `medallion` for the physical layer map, `database-schema` for physical tables. The table below lists every type.

## Level, home and diagram type

Level decides when a diagram is drawn. Scope (system, subsystem, component) decides where it lives: the design whose `diagrams` slot names it, even when drawn at the physical level. The data model's own `diagrams` slot also names the views it draws: one file under `_meta/diagrams/`, named from both slots.

| Level and home | Diagram types |
|---|---|
| Conceptual (product design) | ER nouns and verbs only; Loop (core process cycle); Swimlane (handoffs between human, agents and system across the approval chain) |
| Logical (system design) | ER with fields (`#` key, `→` reference, `#→` key part that is a reference, muted `time:` line, at most 8 entities per view); Architecture (subsystems and connections); DP integration (sources → core → consumers); Data flow (pipeline steps and who does each); State machine (each entity's valid state changes); Timeline (every time role on one walked row); Sequence (point-in-time read and decision path) |
| Physical, system scope (system design) | Medallion (layers, roles, read rules); High-level (a subsystem's data stack); Deployment (where each subsystem runs); DP security matrix (what each principal may do on each component); Dependency graph (package and asset dependencies) |
| Physical, component scope (component design) | Database schema (the component's tables per layer, audit-labelled references where the engine enforces no constraint); State machine when one component owns the lifecycle |
| Below the designs (milestones) | Story map (releases and the cut line) |

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
