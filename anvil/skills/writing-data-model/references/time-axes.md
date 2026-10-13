# Choosing time axes per entity

Ask three questions per entity (Reis): does it need point-in-time lookups, aggregations over time, or an audit trail? The answers choose the axes.

| Axis | Meaning |
|---|---|
| Event time | When something happened in the world. |
| Valid time | When a state was true in the world (valid from, valid to). |
| Decision time | When an outside party decided a change (a split declared before it takes effect). |
| Knowledge time | When the public could know it. The point-in-time axis for anything a model or decision reads. |
| Ingestion time | When it reached the system. |
| Processing time | When the system committed it (transaction time). |

Rules of thumb:

- **Single axis** (processing time): operational records nobody queries as of a past moment.
- **Bitemporal** (valid or event time plus processing time): states and events that need an audit of corrections.
- **Tritemporal** (add knowledge or decision time): financial and regulatory facts, where what was true, what was knowable and what was stored all differ.
- Event sourcing gives event time plus append time for free; current state then needs a maintained view.
- Store instants in UTC with time-zone-aware types; keep the source's original zone separately; use established libraries. Model the fiscal calendar.
- Never confuse the axes: name each the same way in every table.
