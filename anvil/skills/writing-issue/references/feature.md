# Feature issue — body guidance

**Goal:** a predicate naming the capability once it exists (e.g. "agents can fetch a single phase of a SKILL.md without reading the rest"). Describe the end state, not the build task.

No `reproduction_anchor` — nothing exists to reproduce yet, and forcing one is a category error.

Lead `## Problem` with user value: who is blocked, what they cannot do today, and the success criteria — the observable outcome that means the feature delivered. Set the scope boundary in `## Non-goals`.

ACs must name outcomes, not mechanisms. "Users can export a report" is an outcome; "the CLI invokes `pandas.to_csv`" is a mechanism — it belongs in `## Problem` prose if it informs context, not in an AC. If you prescribe a specific tool or runtime mechanism anywhere in the body, run the one command that proves feasibility before the issue lands; if it fails, rewrite as an outcome or split a spike.
