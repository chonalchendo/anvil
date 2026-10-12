# Grafana (banking exec overview) decomposition

Screenshot numbers come from shot.png (1600 px wide; viewport was 1440, so multiply by 0.9 for CSS px). Everything else comes from census.json.

## 1. Stack
- Emotion CSS-in-JS: Observed (`css-xxxx` class names, "emotion" in inspect.json, ev_a6ccb738...).
- React SPA, scenes dashboard: Inferred. The bundle is one 4.4 KB script with one endpoint, `/api/user/auth-tokens/rotate` (ev_3a4b6992...); app code is lazy-loaded.
- Font: Inter only (670 of 670 weighted chars). Observed.
- Canvas charts: Inferred from the screenshot. No theme CSS vars were extracted (`cssVars: []`).

## 2. Layout skeleton
- Left nav 320x900, fixed, collapsible (toggle at bottom). Header 1120x48. Controls row 1120x64 at y=48 (census).
- Content: 24-col grid of panels. Title text panel, then 8 stat tiles (about 126x162 CSS, 9 px gap), then 3 chart panels (about 350x265 CSS). A 3-icon dock sits on the right.
- Sticky: nav, header, controls, legend `thead` (321x24).
- No prose measure; this is a dashboard.

## 3. Type system
- Sizes by weighted text: 14 px/22 px (507), 12 px/18.9 px (99), 28 px/32 px (36).
- Stat values autoscale: 28.7 to 96.7 px (52.8 and 67.5 also seen).
- Ratios: 12 to 14 is 1.17; 14 to 28 is 2.0.
- Weights: 400 (424) and 500 (246) only.
- Letter-spacing: +0.15 px at 14 px, +0.13 px at 12 px, -0.25 px at 28 px.
- Digits look proportional, not tabular (screenshot). `tnum` is unobserved.

## 4. Colour system
- Surfaces: page rgb(17,18,23); panel and nav rgb(24,27,31) (area 967); raised and active rgb(44,47,53); hairline fill rgba(204,204,220,.12).
- Text:
  - Primary rgb(204,204,220): 10.9:1 on panel (computed).
  - Muted rgba(204,204,220,.65), which blends to about rgb(141,142,154): 5.3:1 (computed).
  - Strong rgb(247,248,250).
  - Link rgb(110,159,255).
- Border: 1px rgba(204,204,220,.12) (50 uses); .2 alpha on controls.
- Series and status hues:
  - green 115,191,105
  - orange 255,152,48
  - blue 61,113,217
  - red 242,73,92
  - yellow 242,204,12
  - light blue 87,148,242
- Radius: 6 px (51 uses), 10 px (13), pill (5). No shadows; elevation is surface step plus hairline.

## 5. Spacing and density
- Base is 4 px, working unit 8 px (census: 8 px x161, 4 px x46, 12 px x16, 16 px x10).
- Nav rows are about 29 CSS px, legend rows about 22, buttons 32.
- Chrome above content is 112 px.

## 6. Signature components
1. **Threshold stat tile with sparkline**
   - Problem: "is it passing" across 8 metrics at a glance.
   - Principle: state is pre-attentive as a large area of hue; the number confirms it and the sparkline adds trend at no extra chrome cost.
   - Spec: about 126x162 px. 12 px title bar, coloured body with a translucent area sparkline, white value of 29 to 97 px with units at about half size. Fill is a single threshold hue (green, orange, blue).
   - Anvil jobs: 2 and 4.
2. **Panel chrome**
   - Problem: heterogeneous visualisations that must read as one set.
   - Principle: one container grammar lets content vary without visual noise.
   - Spec: bg rgb(24,27,31) on rgb(17,18,23), 1px hairline, 6 px radius, title 14 px/500 top-left, about 12 px padding.
   - Anvil jobs: 1, 2, 4.
3. **Legend as table**
   - Problem: a chart key that also answers "how much".
   - Principle: put the summary number next to the swatch so the key doubles as a readout.
   - Spec: sticky 24 px header in accent blue, colour dash swatch, right-aligned Mean and Max at 12 px.
   - Anvil jobs: 2, 4 (for example tokens or review rounds per issue).
4. **Breadcrumb plus time-range toolbar**
   - Problem: where am I, and over what window.
   - Principle: scope is always visible and one click from change, in its own band.
   - Spec: 14 px breadcrumb with muted ancestors; 32 px grouped controls (step back, range, step forward, zoom out) with refresh and interval.
   - Anvil jobs: 1, 3. Anvil has no time window, but project and milestone scope map to the same pattern.
5. **Collapsible grouped nav**
   - Problem: deep IA in a persistent rail.
   - Principle: a fixed rail with chevron groups keeps location constant while content changes.
   - Spec: 320 px, 16 px icons, active row fill rgb(44,47,53) with an orange left marker.
   - Anvil jobs: 1, 3.

## 7. Do not copy
- Full-saturation tiles. Hue works but eight solid blocks drown the page. Keep area-of-hue plus number plus trend, scaled down: a pass/fail left bar or badge, with a neutral number.
- Hue used for non-status series (blue, purple). In anvil, hue means pass, fail or in-flight only.
- Autosized stat font; it breaks the type scale. Use a fixed 28 px.
- Truncated tile titles ("Money M...", "Loans Di..."). The label is the identity of the number.
- 112 px of chrome and a 320 px rail; use 240 px or less.
- Brand and edit affordances: logo, Sign in, Chat, AI nav, Edit, share. Anvil is read-only.
- Positive letter-spacing on long prose.

## 8. Unknowns
- full.png is blank, so anything below the fold is unseen.
- Threshold config, so the value-to-hue mapping is unknown.
- Theme tokens, `tnum`, hover, focus and tooltip states.
- Light theme, narrow layout, keyboard behaviour.
- Tile and panel sizes are measured from the screenshot, not the DOM.
