# Deriving the verification strategy

It is keyed by what the component *is* (an API, a pipeline, and a CLI are verified three different ways), not by language. Read the targets off the component design:

- **Direct** = the `## Does not` and `## Invariants` entries as tests, plus the component's failure mode (data integrity: assert a downstream value; idempotency: run twice, assert stable).
- **Indirect** = the component's real entry point (HTTP route, CLI verb, landed table) plus the most-downstream non-proxy observable that proves the change worked. Record any topology limit (for example, prod unreachable from dev).

Ground the approach in the repo, your training data, and online research from recognised experts. When the approach for a component type is not settled, dispatch an `anvil-researcher` subagent before naming the strategy. Test style comes from the language convention, not here.
