# Linear (homepage product mock-ups + /changelog) decomposition

Sources: C-home = linearhome/census.json, C-log = linear/census.json, S = screenshot measurement. Dark colour scheme in both (C-home, C-log).

## 1. Stack
- Next.js: Observed, high confidence, `next-runtime` detector on the changelog (ev_591d36c014703e1499fa12a4fe175b64efd222c2fef68883e6d20378a1c9f1b9). Homepage bundle returned no chunks (ev_4d269401c9da629570bb47ebea4657da0ee0f2390b2c9700e779ae8e65980dc4), so the framework there is Inferred as the same.
- Styling: hashed CSS-module class names (`TZTsQG_header`, `I69PJq_changelogLeft`), Observed in C-home/C-log landmarks (inspect ev_268bfe4a9b3e0470d96ee2557b1dd462d9b3f5e6c104fa1ddd960646ff19c4d4). No utility-class framework seen. Treat as Inferred CSS Modules.
- Fonts: "Inter Variable" is 7499 of 8953 weighted text in C-home and 50131 of 50446 in C-log. "Berkeley Mono" is for code and IDs (1454 on home, 315 on the changelog). Both Observed. No "Inter Display" family appears in the census. The large-heading look comes from size plus tracking, not a separate font.
- Component libs: the homepage `--tweet-*` variables show an embedded tweet widget (Observed, C-home cssVars). Nothing else was identified.

## 2. Layout skeleton
- Site header: 1440x73, fixed or sticky, contains a 72px nav (C-home, C-log). A 32px announcement bar sits at y=72 (C-home, C-log sticky list).
- Marketing container: 1280px wide, x=80..1360 (C-log header landmark). The homepage mock-up window is about 1320px wide (S: x=140..1460 at 1600px).
- Mock-up app window (C-home landmarks): sidebar `nav` 232x704, header strip 1070x44 for the issue title and "1 / 84" pager, floating agent panel 398x48 header. Sidebar and content are split by a 1px border.
- Changelog is a two-column timeline (S). The left column is 296px wide at x=80 and holds a 24px sticky date, an orange dot and a vertical 1px rail. The entry column starts at x=488 (S). Headings and prose run about 620px, from x=488 to roughly 1100 (S). Estimated at 17px that is about 70ch, which is Inferred.
- The date cell is sticky (`changelogLeft`, 296x24, C-log), so the date stays in view while one long entry scrolls past.
- Within an entry, each sub-feature is an h3 (about 24px) followed by prose. The entry ends with collapsed "Fixes / Improvements / API" rows. Each row is a disclosure with a chevron (S).

## 3. Type system
- Sizes seen in C-home: 10, 11, 12, 13, 14, 15, 16, 18, 20, 24, 32, 48, 64, 72. The ratios are irregular, so this is a hand-picked scale. The UI steps are 12 to 13 (1.08), 13 to 15 (1.15) and 15 to 24 (1.6). The display steps are 24 to 32 (1.33), 32 to 48 (1.5) and 48 to 64 (1.33).
- Text weight by size, home (C-home): 15px 2219, 13px 2091, 12px 1712, 14px 1531. Mock-up UI is therefore 12 to 15px and mostly 13px.
- Changelog (C-log): the 17px body has 45427 weighted chars, which is 90 percent of all text. Line-height is 27.2px (1.6). Next are 13px at 2860, 24px h3 at 731 and 32px h2 at 284.
- Weights are 400 body, 510 UI labels and 590 headings (C-home, C-log). 300 appears in the hero. These are variable-font intermediate weights, not 500 or 600.
- Tracking (C-home, C-log):
  - 13px: -0.13px (-0.010em).
  - 15px: -0.165px (-0.011em).
  - 24px: -0.288px (-0.012em).
  - 32px: -0.704px (-0.022em).
  - 48px: -1.056px (-0.022em).
  - 64px: -1.408px (-0.022em).
  - 17px body, 12px and 14px: normal.
- Line-heights: UI 13px uses 19.5px (1.5), 12px uses 16.8px (1.4), 24px uses 31.92px (1.33), 32px uses 36px (1.125) and 48px uses 48px (1.0).
- Numeric features: `tabular-nums` and `lining-nums` are used on counters and pagers (C-home 27 and 13 elements, C-log 35). Stylistic sets `cv01, ss03` apply to 400 elements on home and 1030 on the changelog.
- Mono: Berkeley Mono at 13.6 to 14.9px for inline code and IDs such as `vehicle_state` and `DRV-8852` (S, C-log).

## 4. Colour system
- Surface ladder, as pixel-area weights (C-home): #08090a (15058) is the page background. #0f1011 (4827) is the app panel. Smaller steps are #090a0b, #101112 and #161718 (261, a hover or selected state). Overlays use rgba(255,255,255,.01 / .02 / .05) over those surfaces.
- The changelog is flatter (C-log): #08090a on nearly everything, #141516, #1c1c1f and #23252a for controls.
- Text ladder (both censuses): #f7f8f8 primary (headings, active items), #d0d6e0 body (the changelog uses it for 47709 chars), #8a8f98 secondary, #62666d muted. Contrast on #08090a is computed by me as primary about 17:1, body about 13.6:1, secondary about 6.2:1 and muted about 3.4:1. Muted fails AA for body text and is only fit for decoration or tertiary labels.
- Borders are hairlines of white alpha: .08 is the default (134 uses), .12 is emphasis and .05 is the faintest (C-home). The changelog uses solid rgb(52,52,58) for 764 row dividers.
- Accent is indigo #5e6ad2 (C-home 46, C-log 46), used for focus rings (`0 0 0 3px rgba(94,106,210,.4)`) and primary controls. The CTA is a light pill (#e5e5e6, 9999px radius).
- Status hues (C-home), shown as rgba tints over dark: green (39,166,68,.07), red (243,78,82,.1), yellow #e4f222. Text and glyph tones are #f7bf8b (orange), #f79ce0 (pink), #ffdf9f (yellow), #83dcdc (cyan) and #8fa6ff (indigo).
- Shadows are rare: inset 1px rings (`rgba(255,255,255,.05..08) inset`) do the edge work instead of drop shadows. The one large shadow is `0 2px 32px rgba(0,0,0,.25)`, under the floating panel.

## 5. Spacing and density
- Base unit 4px, with 6px as the dominant half-step. Spacing weights (C-home): 8 (226), 4 (174), 6 (169), 32 (87), 12, 10. C-log: 6 (729), 8 (722), then 12, 16, 20, 24, 32, 48, 56.
- Mock-up sidebar rows are about 30px apart (S: Pulse 601, Inbox 631, My issues 661). Row text is 13px at 20px line-height, with an icon column of 14 to 22px (C-home grid columns).
- The title bar is 44px tall (C-home landmark).
- Radii: 4px chips, 6 to 8px rows and buttons, 12px panels, 9999px pills and avatars (C-home).
- Motion is minimal: 0.1s colour transitions on 187 elements, easing cubic-bezier(.25,.46,.45,.94) (C-home).

## 6. Signature components
1. **Issue row with glyphs (mock-up list).** Solves scanning status across many rows. First principle: status is pre-attentive when it is shape plus hue at a fixed column, not text. Spec: 14 to 16px circular glyph in a fixed icon column (grid columns 14/16/20px), then mono ID (Berkeley Mono 13px, #8a8f98), title at 13px/510 in #d0d6e0, and trailing metadata. Rows run about 30px high (S) and the selected row uses a #161718 fill with a 6 to 8px radius. Serves jobs 2 and 4.
2. **Sidebar with section captions.** Solves orientation in a large graph. First principle: grouping by quiet captions lets the eye skip to a section without any chrome. Spec: 232px wide, 13px items, 12px #8a8f98 section labels ("Workspace", "Favorites") with a small caret, 16px icons, and a selected item shown as a soft fill with no accent bar. Serves jobs 1 and 8.
3. **Issue detail with property rail.** Solves judging an artifact by its key fields. First principle: put the title and body in the reading column and the metadata in a quiet side column, so neither interrupts the other. Spec: header strip of 44px (ID, title, star, "1 / 84" tabular pager), then a 24px title and a 15px body at #d0d6e0, with an "Activity" h3 and a timeline of 13px events. Properties sit in a right column with 12px labels. Serves jobs 4 and 6.
4. **Dated timeline entry (changelog).** Solves reading a long dated feed cold. First principle: a sticky date in a gutter gives a time anchor while the long prose runs in a narrow, readable measure. Spec: 296px gutter, 24px sticky date with an orange dot and a 1px rail, 17px/27.2px body at #d0d6e0 in a measure of about 70ch, h2 32px/590 and h3 24px/590 with -0.012em tracking. Serves jobs 7 and 1.
5. **Collapsed Fixes / Improvements / API rows.** Solves long tails of minor items without burying headlines. First principle: progressive disclosure keeps the headline path short and the detail one click away. Spec: full-width rows with a #34343a hairline divider, 15px label at 510 and a right-aligned chevron (S, C-log border). Serves job 5.
6. **Dark surface ladder with alpha hairlines.** Solves visual hierarchy with no decoration. First principle: elevation is a small luminance step (#08090a, #0f1011, #161718) plus a 1px white-alpha edge, not shadow. Serves every job.

## 7. Do not copy
- The marketing hero (64 to 72px type, tight -0.022em tracking) and the announcement bar. A dense viewer should not use display sizes.
- The 17px/27px changelog body is marketing-width prose. Anvil designs need about 15px with the same 1.6 line-height.
- Muted #62666d at about 3.4:1 contrast. Raise it for any text that carries meaning.
- The CTA pill, the sign-up button, the orange timeline dot as brand colour, the embedded tweet widget, the agent chat panel, and Berkeley Mono (a licensed font).
- Hover-only affordances in the mock-up. A read-only viewer should keep them visible.

## 8. Unknowns
- The exact issue-row geometry and the priority glyph shapes. The mock-ups are truncated in shot.png (the list rows are below the viewport) and I did not zoom into full.png.
- Whether "Inter Display" is loaded. No such family is in the census, so any optical-size switching is hidden inside the variable font (not observed).
- The accent-versus-status colour mapping per state. I only have the tint colours, not which state each belongs to.
- The changelog's body measure in ch. I estimated it from a screenshot, not from computed width.
- Light-theme behaviour. Both pages report a dark colour scheme only.
