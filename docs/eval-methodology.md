# Eval Methodology

How anvil reasons about skill evals. Read when ingesting eval results (the planned `anvil eval`, anvil.0070) or judging skill `confidence`.

Skill `confidence: low | medium | high` is metadata an author asserts. An eval **measures** it: run the skill against fixed prompts, grade each expectation, track the pass-rate over versions. This doc makes that method anvil-native so a dispatched agent can cite the rubric and result schema without reaching the external harness.

**Execution stays external.** anvil records and describes; it does not run or grade. Harnesses that own execution — Anthropic's `skill-creator` plugin (`run_eval.py`, `agents/grader.md`, `references/schemas.md`), or any other — spawn the agent, capture the transcript, and emit the result files below. anvil's stack is Go + markdown, and headless `claude -p` is metered, not subscription-covered ([[issue.anvil.0070.eval-runner-on-agentadapter-run-evals]]). The planned `anvil eval ingest` (anvil.0070) reads the emitted files; the run-cost stays with whoever invokes the harness.

## Grading rubric (per-expectation, evidence-cited)

An eval is a prompt plus a list of **expectations** — verifiable statements like "the output includes X" or "the skill used script Y". A grader judges each one independently against the execution transcript and output files:

- **PASS** — clear, citable evidence the expectation holds *and* reflects genuine task completion, not surface compliance (a file exists *and* has correct content, not just the right filename).
- **FAIL** — no evidence, evidence contradicts it, it can't be verified, or it's satisfied only superficially / by coincidence.

No partial credit; each expectation is binary. The burden of proof is on the expectation — uncertain means FAIL. Every verdict cites the specific transcript quote or output it rests on. A discriminating expectation is one that fails when the skill genuinely fails — a pass-rate built on trivially-satisfiable assertions is false confidence, so the method also critiques weak expectations.

`pass_rate = passed / total` over a version's expectations is the headline confidence signal; its trend across versions is whether the loop is climbing or just accumulating.

## Result schema (what anvil ingests)

anvil parses two files from the harness. Field names below are the ingest contract — see `references/schemas.md` in `skill-creator` for the full producer schema.

`grading.json` — one graded run:

```json
{ "summary": { "passed": 2, "failed": 1, "total": 3, "pass_rate": 0.67 } }
```

`history.json` — version progression across an improvement loop:

```json
{ "skill_name": "pdf", "current_best": "v2",
  "iterations": [ { "version": "v2", "expectation_pass_rate": 0.85, "is_current_best": true } ] }
```

anvil.0070 will ingest `summary.{passed,failed,total,pass_rate}` per run and `iterations[].{version,expectation_pass_rate}` per version into an `eval_runs` table, queryable per skill over time. Keying skill-confidence promotion off that table is downstream, not here.

---

*Rubric and result schema adapted from Anthropic's `skill-creator` plugin (`agents/grader.md`, `references/schemas.md`); execution, transcript capture, A/B baseline, and the eval-viewer remain its responsibility.*
