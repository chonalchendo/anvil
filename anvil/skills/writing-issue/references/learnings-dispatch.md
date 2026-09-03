# Phase 3b — Surface prior learnings, dispatch shape

Dispatch `anvil-learnings-researcher` via the Agent tool's `subagent_type` to pull what the vault already knows about this slice — it reads widely and returns a tight findings list, so the bulk stays out of your window. Build the `<work-context>` from Phases 1–2:

```text
<work-context>
work: <converged/stated proposal, one sentence>
domain: <domain/ tag(s) for this issue>
activity: activity/issue
artifacts: [[milestone.<project>.<slug>]]
files: <paths the fix will likely touch, if known>
</work-context>
Return the findings that genuinely bear on this work, highest-precision first.
```

Fold the return into the issue: on findings, add a `## Prior learnings` section before `## Links` (one bullet per finding, keeping its `[[learning.<id>]]` source and `stale?` flag) and let any non-stale high-confidence one sharpen Problem / Non-goals / Verification. On `Findings: none`, skip the section. `stale?: yes` is a signal to weigh against present evidence, not a directive.
