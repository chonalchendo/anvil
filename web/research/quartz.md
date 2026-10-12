# quartz (quartz.jzhao.xyz) decomposition

## 1. Stack
- Static-site generator output, 20 scripts, 59k AST nodes parsed, no parse failures. Observed: ev_39a4e7ca7f85cae40a4ca4e406b26f3c6575dbbd7ed631fc8056d8a2a9a61dc4 (bundle). Per-widget scripts (`script-N-hash.js`); d3 and mermaid strings present (graph view, diagrams). Observed, same id.
- Server-rendered HTML with small per-widget scripts (search, explorer, graph, popovers). Inferred (Preact SSR, from Quartz's known design; evidence shows no framework fingerprint).
- Fonts Observed (census.json font-family, weighted by text): Source Sans Pro 3539, IBM Plex Mono 552 (code), Schibsted Grotesk 195 (logo, headings). Ids: ev_cabd1f61e3cf61b542d5b5c7009ba964596e8393364c008113c0d74d9c05fb96 (inspect), ev_fd641a60714f6b2d71be6599fb9e77ec944200503eb5afb72661d6dcd90ad3c1 (shot).
- Styling: census cssVars empty, so theme is compiled values, not tokens.

## 2. Layout skeleton
- Three-column grid `320px 790px 320px` (census), viewport 1440. Left sidebar 320x900 at x=0, right sidebar 320x900 at x=1120; both listed sticky/fixed.
- Article 790px at x=325 (census landmarks). Measure: first full.png paragraph line holds about 113 characters (counted on screenshot), roughly 95-100ch. Wide for prose.
- No header bar; content starts about y=100 (shot.png), with 96px spacing value in census. Footer 67px.
- Left: title, search (about 200x32 box, shot.png), theme and reader icons, Explorer. Right: Graph View (about 256x250), Table of Contents, Backlinks stacked.

## 3. Type system
- Sizes (census, weighted): 16px (3583, nearly all text), 13.6 (471, code), 14.4 (81), 15.2 (55), 17.92 (44), 22.4 (25), 28 (27).
- Step ratios: 13.6 to 16 = 1.18, 16 to 17.92 = 1.12, 17.92 to 22.4 = 1.25, 22.4 to 28 = 1.25.
- Weights: 400 (2112), 600 (2042), 700 (132). Half of all text is semibold (links, nav, headings).
- Line-height: 25.6px (1.6) prose and code; 22.4px (1.4) for 448 chars (sidebar/TOC); 24px at 15.2; headings `normal`.
- Letter-spacing `normal` everywhere. No numeric features measured.

## 4. Colour system (dark; census, WCAG computed against bg rgb(22,22,24))
- Surfaces: bg rgb(22,22,24); raised/border rgb(57,54,57); chip/inline-code fill rgba(143,159,169,.15); callout tint rgba(0,191,165,.063) with 1px rgba(0,191,165,.267) border. About 3 levels, tint-on-bg, no shadows in census.
- Text: primary rgb(235,235,236) 15.2:1; body rgb(212,212,212) 12.2:1; muted rgb(106,115,125) 3.75:1 (code comments, line numbers; fails AA); rgb(100,100,100) 3.05:1 (fails).
- Accent: link slate-blue rgb(123,151,170) 5.9:1 (613 chars, all links). Callout teal rgb(0,191,165) 7.7:1. Syntax: rgb(158,203,255) 10.7:1, rgb(121,184,255), violet rgb(179,146,240), red rgb(249,117,131).
- No status hues; callouts are the only semantic colour.

## 5. Spacing and density
- Census spacing: 16px (47), 1.6px (39), 4px (20), 8px (11), 32px (5), 64, 96px. Base 16px with 4/8 below, 32/64/96 above.
- Explorer and TOC row pitch about 26px (shot.png: Advanced y=252, CLI Reference y=278). About 35 rows per 900px.
- Radius 5px (48), 4px (2). Transitions: `color 0.2s ease` (148); rest rare.

## 6. Signature components
1. **Explorer tree**. Problem: see corpus shape and jump. Principle: a folder tree is spatial memory; collapse lets many pages fit one column. Spec: 16px/600 slate-blue, about 26px rows, chevron on folders, bare text on leaves, folders first then alphabetical. Jobs 3, 1.
2. **Wikilink chip**. Problem: tell links from text at reading speed. Principle: links are pre-attentive with colour, weight and a tinted ground, not underline alone. Spec: 16px/600 rgb(123,151,170) on rgba(143,159,169,.15), 5px radius, 0.2s colour transition, arrow glyph on external. Job 8.
3. **Backlinks panel**. Problem: what refers here. Principle: inbound links are as informative as outbound and cost one list. Spec: right column under TOC, heading 16px/600 primary, entries as link text at about 26px. Jobs 8, 4.
4. **Local graph**. Problem: neighbourhood at a glance. Principle: a depth-limited graph centred on the current node gives context without a full map. Spec: about 256x250 box, 1px border, 5px radius, expand icon top right, current node larger and blue-grey, others grey (shot.png); d3 (Observed). Job 8.
5. **Table of contents**. Problem: orient in a long doc. Principle: headings are the document's own index; depth is indent. Spec: right column, collapsible chevron, 22.4px line-height, h3 indented and dimmer. Job 7.
6. **Search box**. Problem: find by name or content. Principle: an always-visible input removes discovery cost. Spec: about 200x32, 1px border, muted placeholder, top of left column. Job 3.

## 7. Do not copy
- "Quartz 5" slate logo, Schibsted brand headings, emoji headings.
- 600 weight on all links and nav: flattens hierarchy.
- 95-100ch measure; use 65-75ch.
- Muted text at 3.75:1 and 3:1 (below AA).
- Graph as decoration: no status encoding, no typed edges.
- Theme toggle and reader-mode icons (anvil is dark-only).

## 8. Unknowns
- Search overlay, results layout, hover popover previews (static shots only).
- Explorer expanded state, active-page highlight, hover states.
- Graph interaction (zoom, depth, global view), node sizes.
- Narrow breakpoints (only 1440 and 1600 captured).
- No CSS variables, so no token names; shadows absent or unmeasured; no overlay colour captured.
