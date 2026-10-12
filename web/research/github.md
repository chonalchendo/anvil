# GitHub Public Roadmap (Projects board) decomposition

Evidence ids: census.json has none; screenshot ev_77f1ff9f...c9230 (shot.json); bundle ev_e98a732f...d90d936 (0 scripts observed); inspect ev_acb4f002...d57aa (scripts.total=0, resources empty). The bundle and inspect evidence yielded no script or vendor data.

## 1. Stack
- Styling and component lib: Primer. Observed in census cssVars (not an evidence id): `--base-size-*`, `--control-medium-size: 2rem`, `--borderRadius-medium: .375rem`, `--text-title-shorthand-*`, `--overlay-*`, `--stack-gap-*`. Primer React vs CSS-modules is Inferred.
- Fonts: Observed "Mona Sans VF" (3213 weighted chars), "Mona Sans" (74), ui-monospace (21, inline `code` in titles), Mona Sans Mono (1). Stack: `"Mona Sans VF", -apple-system, ... sans-serif`.
- Framework: Inferred React SPA (GitHub Projects); no script evidence (0 scripts in bundle and inspect).

## 2. Layout skeleton
- Viewport 1440x900 (census) / 1600x913 (shot). Global header `header[role=banner]` 1440x72, black (0,0,0).
- Below it, full-width, no centred column:
  - Title row 72-120 (48px): project title 20px/600, then Insights and "..." buttons right.
  - View tabs `nav` 16,120 849x37.
  - Filter bar about 32px high (measured on shot, y168-200), 16px side margin.
  - Board starts y~221.
- Board: 4 columns, 349px wide, 9px gap, stride 358px, 16px left margin (shot measurement). Columns are fixed-width and scroll horizontally. A 5th column would be off-screen.
- Column: header 49px (radio-circle status icon, name 14px/600, count pill), then cards stacked with about 9px inner padding and about 8px gap. Each column scrolls vertically on its own. Header and filter bar stay put (inner scroll), so no sticky elements show in census (only a 2px progress loader).

## 3. Type system
- Sizes by weighted char count: 14px (1887), 12px (1331), 16px (54), 20px (21), 24px (15), 11px (1). Scale 12/14/16/20/24 gives ratios of 1.17, 1.14, 1.25, 1.20, about 1.2 average. Nearly everything sits at two sizes (14 and 12), so hierarchy comes from weight and colour, not size.
- Line-height: 14px/18.2px (1.3) for card titles and body, 12px/18px (1.5) and 12px/15.6px (1.3) for pills and metadata, 20/30, 24/36 (1.5) for headings.

## 4. Colour system
- Surfaces: page bg rgb(1,4,9) `#010409` (2045 weight). Raised: rgb(13,17,23) `#0d1117` (1075), used for cards and columns. Overlay/hover: rgb(33,40,48), rgba(61,68,77,0.7). Header is pure black. Ladder is #010409 -> #0d1117 -> #212830, steps of about 7 and 20 L.
- Text: primary rgb(240,246,252) `#f0f6fc` (1806), contrast 18.9:1 on bg, 17.4:1 on raised. Muted rgb(145,152,161) `#9198a1` (307): 7.05:1 on #010409, 6.50:1 on #0d1117. Both pass AA and AAA-large.
- Pills use a tinted-fill recipe: text colour at full strength, fill the same hue at 0.10-0.18 alpha, border the same hue at 0.3-0.4 alpha. Hues:
  - teal 46,205,214 (the most frequent, "Teams Gain Velocity", "Team")
  - green 63,185,80 (GA, issue-open icon)
  - amber 210,153,34 (Public Preview)
  - lime 153,221,136 (Shipped, "Preview")
  - grey 237,237,237 (Paused, "GHES x.y", "Public Preview")
  - ice 204,240,255 and blue 68,147,248
  - purple 171,125,248 (a theme label)

## 5. Spacing and density
- Base unit 4px, scale 4/8/12/16 (weighted: 8px 272, 4px 84, 12px 49, 6px 41, 16px 20). Primer `--base-size-*` is the 4px ladder.

## 6. Signature components
1. **Status column board.** Problem: see what is up next, being explored, shipped, paused at once. First principle: position encodes state, so state is read by location, not by parsing text. Spec: 349px columns, 49px header with an outlined-circle icon, name 14px/600, count pill 12px muted (43/19/35/7 here; the total is 104). Jobs 1, 2.
2. **Card anatomy.** Problem: a card must be scannable and identifiable in under a second. First principle: put identity (type icon plus `repo #id`, 12px muted) above the title (14px/600 primary), so the eye reads id then title then tags. Spec: 332px wide, #0d1117 bg, 1px #3d444d border, 6px radius, 9px padding, three tiers. Jobs 2, 4.
3. **Tinted-fill label pills.** Problem: many tags per card without visual noise. First principle: low-alpha fill with a same-hue border reads as category while keeping text legible at 6+:1 contrast. Spec: 12px/600, 20px tall, full radius, fill alpha 0.10-0.18, border alpha 0.3-0.4. Weakness: up to 6 pills per card and hue is arbitrary. Jobs 4, 8.
4. **Filter query bar.** Problem: slice the board without a form UI. First principle: a typed query with syntax colouring is both the control and a readable record of the current state. Spec: `is:open has:status -archive:<=@today`, qualifiers white, values blue #4493f8, a result count badge (104), a clear (x) button, and a View menu on the right. Jobs 3, 2.
5. **View tabs.** Problem: save several named slices of one dataset. First principle: each tab is a stored query plus layout, so the tab is the bookmark. Spec: 5 tabs (All Items, General Availability Ite[ms], Public Previews, Server (GHES), Recently Shipped) at 14px with a board-layout icon. The active tab has a bottom accent and a raised bg. Long names truncate ("Ite"). Jobs 1, 3.

## 7. Do not copy
- Marketing global header (Platform, Solutions, Pricing, sign in): 72px of brand chrome.
- Pill overload: up to 6 pills with arbitrary hues. Anvil should use few, semantic hues.

## 8. Unknowns
- No bundle or script evidence, so framework, state and virtualization are unproven.
- Pill row heights are measured only by eye on the screenshot.

