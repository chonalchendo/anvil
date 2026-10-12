import { execFileSync } from "node:child_process";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";

// A scaffolded vault seeded with the shapes the server must handle: a live
// milestone, an escalated loose issue, a broken spine edge, a session, hostile
// markdown and two diagrams. `anvil` builds the index, so the server reads
// exactly what the CLI would write.
const files: Record<string, string> = {
  "05-product-designs/product-design.demo.md": `---
type: product-design
title: Demo — product design
description: A fixture project
project: demo
status: active
created: "2026-10-01"
updated: "2026-10-01"
diagrams:
  - demo-flow
tags: []
---
## Milestones

Nothing yet.
`,
  "85-milestones/milestone.demo.first.md": `---
type: milestone
title: First milestone
description: The first one
project: demo
status: in-progress
created: "2026-10-01"
updated: "2026-10-02"
approved: "2026-10-01"
product_design: "[[product-design.demo]]"
tags: []
---
## Status

Measured: \`2026-10-02, at abc1234. Every predicate passes.\`
`,
  "70-issues/issue.demo.0001.live-issue.md": `---
type: issue
title: Live issue
description: In progress
project: demo
status: in-progress
owner: tester
severity: medium
created: "2026-10-01"
updated: "2026-10-03"
milestone: "[[milestone.demo.first]]"
verified_verdict: pass
cost_tokens: 1234567
cost_rounds: 2
external_links:
  - https://github.com/x/y/pull/7
  - https://example.com/notes
tags: []
---
## Problem

See [[decision.demo-topic.0001-a-decision]]. A [bad](javascript:alert(1)) link, <script>alert(2)</script> raw html, and <javascript:alert(3)>.

A [ref link][ref] and an ![image](javascript:alert(5)).

[ref]: javascript:alert(4)
`,
  "70-issues/issue.demo.0002.escalated-loose.md": `---
type: issue
title: Escalated loose issue
description: No milestone
project: demo
status: escalated
created: "2026-10-01"
updated: "2026-10-03"
tags: []
---
## Problem

Loose.
`,
  "70-issues/issue.demo.0003.broken-edge.md": `---
type: issue
title: Broken edge issue
description: Milestone missing
project: demo
status: open
created: "2026-10-01"
updated: "2026-10-03"
milestone: "[[milestone.demo.gone]]"
tags: []
---
## Problem

Broken.
`,
  "30-decisions/decision.demo-topic.0001-a-decision.md": `---
type: decision
title: A decision
description: Decided
project: demo
status: accepted
created: "2026-10-01"
updated: "2026-10-01"
tags: []
---
## Decision

Yes.
`,
  "10-sessions/abc.md": `---
type: session
title: A session
status: closed
created: "2026-10-01"
updated: "2026-10-01"
---
`,
  "_meta/diagrams/demo-flow.html": "<!doctype html><html><body><svg width='400' height='200'></svg></body></html>",
  "_meta/diagrams/demo-evil.html": "<!doctype html><html><body><script>alert(1)</script></body></html>",
};

export function seedVault(): string {
  const root = join(mkdtempSync(join(tmpdir(), "anvil-web-")), "vault");
  execFileSync("anvil", ["init", root], { stdio: "ignore" });
  for (const [rel, body] of Object.entries(files)) {
    mkdirSync(dirname(join(root, rel)), { recursive: true });
    writeFileSync(join(root, rel), body);
  }
  execFileSync("anvil", ["reindex"], { env: { ...process.env, ANVIL_VAULT: root }, stdio: "ignore" });
  return root;
}

if (import.meta.url === `file://${process.argv[1]}`) console.log(seedVault());
