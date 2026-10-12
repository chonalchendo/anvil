# shadcn/ui Command docs: decomposition

## 1. Stack
- Next.js + React: Observed (bundle.json ev_58714ea80e44ed8d7eab44cb76621df9087a5b0e4edbdc0d72f3cdf53cb419a6, vendor_fingerprint high, 126 scripts, 0 parse failures).
- cmdk (Dia/pacocoursey): Observed. inspect.json (ev_0267f11e55db58b1d434f13df7bd746acdc59c1a3b2326ac2998d9de31878f91) contains `cmdk-root, -input, -list, -list-sizer, -group, -group-heading, -group-items, -item, -separator, -label`. The page text says "uses the cmdk component by Dia".
- shadcn wrapper: Observed. `data-slot` attributes (393 hits). Variants Base UI / React Aria / Radix UI on the page; Radix and base-ui strings are present.
- Tailwind v4: Inferred. Utility classes such as `w-(--sidebar-width)` and `top-[calc(var(--header-height)+1px)]`, plus `--color-stone-500` vars. There is no Tailwind string in the inspect dump.
- Fonts: Geist (2347 weighted chars), Geist Mono (969), ui-monospace (174), Noto Sans Arabic (96, RTL demo). Observed (census). Ligatures are off for code (`calt 0, liga 0`).

## 2. Layout skeleton
- Sticky header: 1440x64, `bg-background`, z-50. Top nav is 559x32 at x=24 (census).
- Grid is `288px 1136px` (census). Left sidebar is 288 wide, sticky, starting at y=74. The right "On This Page" rail is 288 wide at x=1144, sticky at `top: header+1px`, 810 high.
- Content column is 640px (grid-template-columns 638/640 px, 8 hits), centred between rails. The shot shows its left edge at x=480 and right at 1120. At 14-15px Geist that gives roughly 75-85ch. This is estimated, not measured.

## 3. Type system
- Sizes (census, by text length): 14px (1734), 12.8px (1098, sidebar and ToC), 15px (318, prose), 16px (175), 12px (143), 18.75px (89, h2), 12.75px (22), 30px (7, h1).
- Ratios: 1.07, 1.09, 1.07, then 1.25 and 1.6 to headings. No modular ratio.
- Weights 400/500/600 (1918/1537/131). Letter-spacing normal, except h1 -0.75px and 12px uppercase labels +1.2px. tabular-nums once.
- Line-heights: 14/20 (UI rows), 14/24.5 (1.75 for docs prose), 15/26.25 (1.75), 12.8/18.3 (1.43), 18.75/26.25, 30/36.

## 4. Colour system (dark, `.dark`, Observed in cssVars)
- Surface ladder:
  - `--background` #0a0a0a (census bg lab 2.75, 7971 area).
  - `--card` / `--popover` / `--sidebar` #171717.
  - `--secondary` / `--muted` / `--accent` #262626 (accent #404040 in a variant).
  - `--surface` (code blocks) #161616.
  - Raised steps are about 9 L* units each.
- Text ladder:
  - `--foreground` #fafafa (lab 98.26, 1812 weight).
  - `--muted-foreground` #a1a1a1 (lab 66, 650 weight).
  - Code number gutter #a4a4a4.
- Borders: `--border` #ffffff1a (white at 10%), `--input` #ffffff26 (15%). Hairlines are alpha-white, so they work on any surface.
- Accent: none by default. Primary is monochrome. The theme swatches (blue, lime and others) only swap `--primary`, `--ring` and `--sidebar-primary`. The only blue is `--sidebar-primary` #1447e6.
- Contrast (computed): dark muted-fg #a1a1a1 on #0a0a0a is 7.7:1. #a1a1a1 on #262626 is about 5.5:1. Light #737373 on #fff is 4.7:1.
- `--radius` .625rem (10px). Radii in use: 8px (144), 10, 6, 18, 14. This follows radius-lg minus steps.

## 5. Spacing and density
- Base unit is 4px, with 8px dominant (562 hits). Next are 6 (81), 4, 10, 16, 24, 40, 48.
- Sidebar items are 32px apart (shot: Bubble at y=128, Button at y=160). Item text is 12.8px at a 32px row, so about 2.5x line-height. ToC rows are about 26px apart (y=137, 163, 190).

## 6. Signature components
1. **Command palette (cmdk).**
   - Anatomy (docs "Composition" tree): `Command > CommandInput + CommandList > (CommandEmpty, CommandGroup[heading] > CommandItem[+CommandShortcut], CommandSeparator)`.
   - Problem: find and jump to anything by typing.
   - First principle: focus stays in the input while the active row moves by arrow key (`aria-activedescendant` model). The mouse is never needed, and filtering and selection are decoupled from focus.
   - Spec:
     - Container max-w-sm (about 384px), `rounded-lg border`, popover bg #171717 (shot 608-992 wide).
     - Input has a leading search icon and a muted placeholder.
     - Group headings are 12px muted. The active item gets `bg-accent` (#262626 or #404040).
     - Items have a 16px icon on the left. Shortcuts are right-aligned, muted, with ⌘P-style glyphs (shot).
     - Disabled items are dimmed (Calculator).
     - The separator is a 1px border.
   - Jobs: 3 (find), 8 (jump to linked artifacts).
2. **Token pairing system (`--x` / `--x-foreground`).**
   - Problem: theming and consistency.
   - Principle: one semantic name per role, with every surface paired to its legible text colour. Swapping the `:root` and `.dark` blocks re-themes everything.
3. **Docs shell (3-column sticky).**
   - Problem: orienting in a long reference page.
   - Principle: persistent sibling nav on the left and in-page position on the right keep the reader oriented without scrolling back.
   - Spec: sidebar 288 and ToC 288, both sticky. Active ToC item gets weight and colour. The active sidebar item is a pill with 8px radius, `bg-accent`.
   - Jobs: 1, 7.
4. **Preview-over-code block.**
   - Problem: showing what a thing is and how it is written.
   - Principle: render and source live in one bordered frame, with the code collapsed behind a "View Code" fade.
   - Spec: 640px wide, radius 18px frame, code surface #161616, Geist Mono 12.8px, gutter numbers #a4a4a4 (shot).
   - Jobs: 7.

## 7. Do not copy
- The theme-swatch system (a dozen primary hues) and the Vercel promo card in the ToC rail.
- The monochrome primary: it has no status hues beyond `--destructive`.

## 8. Unknowns
- Exact `CommandItem` height and padding. The census is page-wide, not per component, and the palettes are inside closed demo dialogs.
- Which of the several `:root` / `.dark` var sets applies to the live page. The cssVars list contains multiple theme blocks, and the neutral dark one is assumed.
