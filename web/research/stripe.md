# stripe (docs.stripe.com/payments/checkout) decomposition

Scope: this page shows no right TOC, callout or code block; the shell is left nav + article only. Those parts are Unknown.

## 1. Stack
- Markdoc content pipeline: Observed. The DOM attribute list in inspect.json carries Markdoc-related names and `data-toc-text` (ev_7372f8231926fdea24fdfbd51ff1967097d974df61325e5a7d804692a8fe72b0); headings carry their TOC label as an attribute, so the on-page TOC is generated from headings.
- One 587,732-byte script, one route `/payments/checkout`, no chunk edges: Observed (ev_eb5f48e5a1eeb9bcec25fbbf0c2fb3252b16481257f485d7626b854682ea2a59). Looks like a single server-rendered bundle with client hydration; framework name Inferred, not fingerprinted.
- Styling: no CSS custom properties (census `cssVars: []`), so values are compiled/hard-coded: Observed (census.json). Likely CSS-in-JS (Sail); Inferred.
- Fonts: computed family resolves to `-apple-system` stack for 4915 weighted units; `Source Code Pro` for 1 unit (the only mono seen). Observed (census.json). Wordmark is an image/SVG.

## 2. Layout skeleton
- Top bar ~112px tall total (screenshot): row 1 ~64px (logo, centred search 202px wide + "Ask AI", Sign in), row 2 nav ~37px (landmark `nav 24,75 1205x37`). Background tinted (#f4f7fa-ish).
- Left sidebar: ~280px wide, 1px right border at x=279 (shot.png). Independent scroll; locale footer ("United Kingdom / English") pinned at its bottom, collapse button top right.
- Article: starts x=401 at 1600px viewport, width 1079px (census landmark `article 401,180 1079x2612`); at 1440px it runs x=328..1392 (full.png). Gutter left of article ~120px.
- Measure: a 16px/26px paragraph holds ~107 characters per line (measured on shot.png: "You can use three payment UIs ... Stripe hosts for each"). That is far past 75ch; this page lets prose run wide because it is a landing page.
- Breadcrumb nav 36px tall above h1 (`nav 401,144 1079x36`). Footer inside article column at y=2840.
- Sticky: header and sidebar (inferred from screenshots staying fixed under inner scroll); census lists no sticky entries.

## 3. Type system
- Family: system sans only; mono is Source Code Pro.
- Sizes by weighted use: 14px (2489), 15px (901), 16px (731), 12px (407), 13px (149), 24px (104), 21px (92), 20px (22), 32px (21).
- Scale: 12, 13, 14, 15, 16 are 1.07-1.08 steps (a dense body cluster); then 20, 21, 24, 32 (ratios 1.05, 1.14, 1.33). Display-to-body: 32/16 = 2.0.
- Line-heights: 14/20 (1.43), 15/21 (1.40), 16/24 (1.5) and 16/26 (1.625, long prose), 12/16 (1.33), 24/32 (1.33), 32/40 (1.25), 20/28 (1.4).
- Weights: 400 (3658), 500 (716), 600 (355), 700 (187). h1 700, section h2 ~24px 700, h3 ~21px 600 (screenshot).
- Letter-spacing: `normal` at every size; uppercase labels (13-14px, 9 uses) carry sidebar group headings ("ONLINE PAYMENTS").
- Numerics: `font-feature-settings "pnum"` / proportional-nums on 35 elements.
- Most text sits at 14px (nav, sidebar, meta) and 15px (body copy, colour #4e566d) with 16px for lead text.

## 4. Colour system (light theme; Stripe has no dark docs theme on this page)
- Surfaces: #ffffff (5418 units), #f4f7fa (256, header), #f7fafc (161), #131f41 navy (82, cookie bar).
- Text ladder: #1a1f36 headings (303), #414552 / #3c4257 body (988 / 301), #4e566d prose (901), #50617a muted (926), #1a2c44 (653).
- WCAG on white: #1a1f36 16.2:1, #414552 9.6:1, #4e566d 7.3:1, muted #50617a 6.3:1 (6.0:1 on #f7fafc). Muted text passes AA and AAA-large comfortably.
- Shadows: near none; ring `0 0 0 1px #d4deea`.
- Borders: #ecf1f6 (34), #e3e8ee (29), #d4deea (2). 1px hairlines; separation by hairline, not shadow.
- Accent: #5469d4 links/active (655 units, 4.83:1 on white), #6c8eef lighter (3.1:1), violet #533afd/#4e11e2 brand moments. Status: green #217005 text, #eafcdd fill, #a8f170 border; violet #e0d9fb fill, #c3b6fb border (callout-like tinted chips, census).

## 5. Spacing and density
- Base unit 4px; scale used: 4, 8 (251 uses), 12, 16 (109), 20, 24, 32, 48. Odd 6.79px is a fractional artefact.
- Sidebar rows: 28px pitch in nested levels (y=385, 413, 441 on shot.png); wrapped lines 20px.
- Section rhythm: h2 to paragraph ~36px; hero rule to next section ~60px (shot.png).
- Radii 4px (58), 6px (7), 8px (3): nearly square.

## 6. Signature components
1. **Collapsing tree sidebar** (reader: where am I among ~hundreds of pages). First principle: hierarchy shown by indent + chevron + one active-path emphasis lets you hold location with zero reading. Spec: 280px, 14px/20px, rows ~28px, indent 8px per level (x=28 -> 36), active parent 600 weight dark, active leaf #5469d4, uppercase 13px group labels with 1px rule above. Jobs 3, 8.
2. **Page-action strip** ("Ask about this page / Copy for LLM / View as Markdown"). Principle: put the agent-consumable form of a document one click from the human form. 13px 500, icon + label, 1px dividers, under the lede. Jobs 7 (and agent handoff).
3. **Breadcrumb + h1 + lede stack**. Principle: orientation then identity then promise, three type steps (14/32/21) in 96px. h1 32/40 700; lede ~24px 400 #1a1f36. Job 1, 7.
4. **Hairline-section prose with hero figure**: 1px #ecf1f6 rules between sections, two-column (text left, illustration right) at 1079px. Principle: whitespace + rules replace card chrome. Job 7.
5. **On-page TOC, callouts, code blocks**: not rendered here; see Unknowns.

## 7. Do not copy
- Light-only palette, brand violet, hero illustration, "Ask AI", cookie banner, locale picker, 107ch line length (cap at 70-75ch), two-tier top nav.

## 8. Unknowns
- Right-hand TOC: absent at 1440 and 1600 viewports; only the `data-toc-text` attribute hints at it. Its width, scroll-spy styling and breakpoint cannot be measured.
- Callout and code-block styling: not on screen. Only tinted green/violet chips in the census are evidence (#eafcdd, #e0d9fb with matching borders); code colours and 4-5 syntax hues unseen. Source Code Pro size unknown.
