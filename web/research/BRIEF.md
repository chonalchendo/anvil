# Decomposition brief (shared by every target agent)

FORBIDDEN: never run `anvil transition`, `anvil set`, `anvil create`, `anvil link` or any vault write. Do not edit /Users/conal/Development/anvil*. Write only your one output file.

## Goal
Reverse engineer ONE best-in-class web UI into first-principle design components, so a rebuild of the anvil UI can combine the best components from several targets.

## Anvil in one paragraph
Anvil's vault is a typed graph of markdown artifacts that agents write and a human reads: product/system/component designs (long docs with diagrams), milestones (with acceptance "Measured:" verdicts), issues (status, owner, verdict pass/fail, PR, review rounds, token cost), decisions, learnings (confidence), threads, inbox notes. The UI is read-only, localhost, dark. Reader jobs: (1) orient on a project: what it is, what is moving, what is next; (2) see in-flight work and whether it passes; (3) find anything by name or content (palette + full-text); (4) judge an artifact by its key fields; (5) distil cross-project knowledge; (6) read an issue's context stack (ordered layers); (7) read a long design cold with diagrams; (8) see links between artifacts.

## Evidence in /tmp/anvil-ui-re/<target>/
- `shot.png`: REA viewport screenshot (1600x913). `full.png`: full-page 1440-wide screenshot (may be viewport-only for inner-scroll apps). LOOK AT BOTH with the Read tool.
- `census.json`: computed-style census, values weighted by text length or area: font-family/size/weight/line-height/letter-spacing/colour, backgrounds, borders, radii, shadows, spacing, grids, sticky/fixed elements, transitions, landmark geometry, CSS custom properties.
- `bundle.json`: REA analyze-web-bundle (vendor fingerprints, chunks, routes). `inspect.json`: REA inspect-web-page (DOM, accessibility nodes, resources); it is large: query it with python3 -I or jq, never cat it whole.
Cite REA `evidence_id` values (the `ev_...` fields at the top of each REA JSON) for stack claims.

## Output: /tmp/anvil-ui-re/<target>/decomposition.md, under 900 words, exactly these sections
1. **Stack**: framework, styling, component libs, fonts. Mark each Observed (with evidence id) or Inferred.
2. **Layout skeleton**: columns and their px widths, measure (body line length in ch), sticky regions, header height.
3. **Type system**: families, the size scale (list sizes; compute the ratio between steps), weights, line-heights, letter-spacing by size, numeric features. Which sizes carry most text.
4. **Colour system**: surface ladder (bg → raised → overlay), text ladder (primary/secondary/muted with values), borders, accent, status hues. Note contrast of muted text on bg (compute WCAG ratio).
5. **Spacing and density**: base unit, the scale actually used, row heights of lists/tables.
6. **Signature components** (3–6): for each — name; the reader problem it solves; the FIRST PRINCIPLE behind it (why it works, in one sentence, e.g. "status is pre-attentive when it is shape+hue, not text"); the measured spec (sizes, colours, spacing); which anvil reader job(s) it serves.
7. **Do not copy**: what in this UI is brand, marketing or wrong for a dense read-only knowledge viewer.
8. **Unknowns**: what the evidence could not show.
Be quantitative. Every number must come from census.json, the REA JSON, or a measurement on the screenshot (say which).
