# Anvil web — design synthesis

Each part of this UI comes from one reverse-engineered source. The evidence is in `research/<source>.md`. Each file decomposes REA captures (DOM, accessibility tree, bundle fingerprints, screenshots) plus a computed-style census.

## Shared first principles

The sources agree on these. Every component obeys them.

1. **Elevation is a small luminance step plus a 1px white-alpha hairline, never a shadow.** Linear (#08090a → #0f1011 → #161718), shadcn (#0a0a0a → #171717 → #262626), Grafana, GitHub.
2. **Hue means state, and only state.** Status reads pre-attentively as shape plus hue at a fixed column. Linear glyphs; GitHub status columns. Grafana's series hues and GitHub's arbitrary label hues are the counter-examples.
3. **One dense UI size; hierarchy from weight and colour, not size.** Linear 13px; GitHub 14 and 12px carry 95% of text; shadcn 14px.
4. **Prose gets a measure, not the window.** Every source that let prose run wide (Stripe 107ch, Quartz 100ch) is a do-not-copy; Linear's changelog holds about 70ch at 1.6 line height.
5. **Text that carries meaning stays at 4.5:1 or more.** Linear's #62666d (3.4:1) and Quartz's muted (3.75:1) fail; GitHub's #9198a1 (7:1) is the model.
6. **Read-only means no hover-only affordances.** Everything visible at rest.

## Tokens

| Role | Value | Source | Contrast on bg / raised |
|---|---|---|---|
| bg | `#08090a` | Linear | — |
| panel | `#0f1011` | Linear | — |
| raised | `#16171a` | Linear | — |
| hover | `#1c1d21` | — | — |
| line | `rgb(255 255 255 / .08)`, strong `/ .12` | Linear, shadcn | — |
| fg | `#f7f8f8` | Linear | 18.7 / 16.9 |
| body | `#d0d6e0` | Linear | 13.6 / 12.3 |
| dim | `#8a8f98` | Linear | 6.1 / 5.5 |
| faint | `#80858d` | raised from Linear's #62666d | 5.4 / 4.8 |
| accent (focus, selection) | `#7c87e8` | Linear #5e6ad2, lifted for text use | 6.2 / 5.5 |
| status: open / in progress / done / pass / fail / escalated / abandoned | `#9aa0a8` / `#f2c94c` / `#7c87e8` / `#4cb782` / `#eb5757` / `#f2994a` / `#80858d` | Linear state colours | all ≥ 4.5 on every surface |

Type: Inter Variable with `cv01, ss03`, weights 400 / 510 / 590 (Linear's variable steps). Mono: Geist Mono for ids, SHAs and code. Scale: 11 (caps captions, +0.04em), 12, 13 (UI default, 1.5), 15 (prose, 1.6), 18, 24 (page title, −0.012em). No display sizes. `tabular-nums` on every count.

Space: 4px base; rows 32px (lists), 28px (sidebar), header 44px (Linear's title bar). Radii: 4 (pills), 6 (rows, buttons), 10 (panels).

## Components and their source

| Component | Source | First principle | Reader job |
|---|---|---|---|
| App shell: 232px sidebar with quiet caps captions, soft-fill selection, no accent bar | Linear | Quiet captions let the eye skip to a section with no chrome | orient |
| Issue row: status glyph column, mono id, title, then right-aligned judge cells (verdict, PR, rounds, tokens) in tabular nums | Linear row + Grafana legend-as-table | A fixed glyph column makes status pre-attentive; numbers beside the label make the row its own readout | in-flight, judge |
| Milestone group: fold with progress meter and "N of M" | Linear list groups + Grafana stat principle, scaled down | Area of hue states pass or fail; the number confirms it. A thin bar replaces the full tile | in-flight |
| Board view: status columns with counts | GitHub Projects | Position encodes state, so state reads by location | in-flight |
| View tabs: List / Board | GitHub | The tab is a stored query plus layout | orient |
| Tinted pills: fill 12% alpha, border 30%, text full | GitHub | Low-alpha fill reads as category and keeps 6:1 text | judge |
| Artifact page: breadcrumb → title → lede stack, prose at 68ch, property rail on the right | Stripe stack + Linear issue detail | Orientation, then identity, then promise; metadata in a quiet side column does not interrupt reading | judge, read |
| Rail: judge fields, sticky TOC with scroll-spy, backlinks, local graph | Linear rail + Quartz | Inbound links are as informative as outbound; a depth-1 graph gives neighbourhood without a full map | links |
| Local graph: status-coloured nodes, typed edges | Quartz, fixed | Quartz's graph is decoration; anvil's encodes status and relation | links |
| Knowledge timeline: sticky date gutter, entries in a 70ch column | Linear changelog | A sticky date anchors time while long prose scrolls | distil |
| Command palette: focus stays in the input, groups, right-aligned type hint, last row falls through to body search | shadcn / cmdk | The `aria-activedescendant` model: typing and choosing never fight for focus | find |
| Context stack: ordered folded layers with size | Linear's collapsed rows | Progressive disclosure keeps the path short and detail one click away | stack |

## Do not copy (collected)

Marketing heroes and display type; brand colours; full-saturation stat tiles; arbitrary label hues; light themes; 100ch prose; text under 4.5:1; edit, share and sign-in chrome; 320px rails.
