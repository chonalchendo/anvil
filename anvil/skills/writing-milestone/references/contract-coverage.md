# Contract coverage cadence

Name the component families this milestone owns (read them off the acceptance criteria), then see which already have a governing contract:

```bash
anvil list contract --json
```

A contract **accretes from building**, so it is authored at the start of the milestone that owns the family (`docs/system-design.md`, contract cadence). For an **extremely obvious** uncovered family — one this milestone plainly owns and builds against — surface the gap and, on the user's confirmation, fire `writing-contract` (author mode) for it. Skip silently when every family the milestone owns is already covered, or when the gap is ambiguous — never author speculatively. This is the authoring end of the cadence; `writing-issue` Phase 4b stays link-only.

**REQUIRED SUB-SKILL (on confirmed gap only):** Use `writing-contract`.
