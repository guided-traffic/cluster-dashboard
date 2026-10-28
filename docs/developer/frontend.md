# Frontend

The page: Angular 22.2, standalone, zoneless, signals, no component library
([ADR 0005](../adr/0005-cogpits-dark-cockpit-on-the-web-without-a-component-library.md) D2). It reads
one answer — `GET /api/v1/dashboard` — and draws it in the look of
[ADR 0012](../adr/0012-a-technical-editorial-look-the-apps-beside-the-bookmarks.md): neutral grounds,
hairlines, square corners, colour only for what is not healthy. Every decision about an item (its
icon, its fallback, its badges, what is hidden) was made by the backend.

## The tree

| Path | Responsibility |
|---|---|
| [src/app/app.ts](../../frontend/src/app/app.ts) | The shell: the top bar — the mark and `ui.title` (the page's `h1`, also the document's title), the backend's version, the theme control — over the dashboard |
| [src/app/core/dashboard.service.ts](../../frontend/src/app/core/dashboard.service.ts) | Asks for the dashboard on start, every `ui.refresh_seconds`, at once when the page becomes visible again, never while it is hidden; keeps the last answer when a refresh fails |
| [src/app/core/theme.service.ts](../../frontend/src/app/core/theme.service.ts) | The colour scheme: the stored choice, else `ui.default_theme` once the dashboard has answered, else `system` |
| [src/app/core/clock.ts](../../frontend/src/app/core/clock.ts) | The greeting's time of day, the date and the time, to the minute |
| [src/app/features/dashboard/](../../frontend/src/app/features/dashboard/dashboard.ts) | The page: the overview with the summary, the apps beside the bookmarks, the footer; the horizon where there is nothing to show |
| [src/app/shared/](../../frontend/src/app/shared) | `item-icon` (a glyph as a mask, a logo as an image), `badge` (a square at a corner or a chip), `group-head` (a group's name and count), `theme-control` |
| [src/app/design/tokens.css](../../frontend/src/app/design/tokens.css) | Every colour as a `light-dark()` pair — surfaces, text, hairlines, the accent, status, badge tones — the four faces, the frame and the rhythm |
| [src/styles.css](../../frontend/src/styles.css) | The page's ground and the two global classes: `.hud` (a mono label) and `.visually-hidden` |
| [src/testing/](../../frontend/src/testing/dashboard.ts) | Fixtures of the dashboard answer, for the specs only |
| `src/app/api/` | The client ng-openapi-gen writes (`make frontend-generate`), never edited, never formatted |

## The page's layout

```text
┌ top bar ── mark + title (h1) │ … │ version │ ◐ │ ☀ │ ☾ ┐   sticky
├ frame, ≤ 1280 px, hairline edges ───────────────────────┤
│ 01 / OVERVIEW                          date · time       │   sticky bar
│ Good *evening*, Hans.                                    │   or ui.title
│ 13 apps in 3 groups. 1 down, 1 degraded.  ■■■■■□         │   ui.showSummary
│ APPS 13 │ HEALTHY 10 │ DEGRADED 1 │ DOWN 1 │ UNKNOWN 1   │
├──────────────────────────────────┬──────────────────────┤
│ 02 / APPS        13 apps · 3 gr. │ 03 / BOOKMARKS 8 links│   sticky bars
│ MONITORING                     3 │ REPOSITORIES        3 │   group heads
│ tile │ tile │ tile               │ 01 ▢ dash           ↗ │
├──────────────────────────────────┴──────────────────────┤
│ Watching Ingresses in …             Refresh every 30 s   │   footer
└─────────────────────────────────────────────────────────┘
```

The split is a wrapping flex row: the apps `flex: 999 1 560px`, the bookmarks `flex: 1 1 320px`, a
1-pixel gap over the hairline colour as the divider. Below 881 pixels it wraps, the apps first. Without
bookmarks the apps take the frame's width.

## The page's rules

- **Quiet when healthy.** A healthy tile draws no colour: its ready count `2/2` in faint text.
  Degraded: a 2-pixel amber line along the top edge and `2 of 3 ready` in amber; down: a coral line,
  the coral tint `--down-tint` and `down`; unknown: no word. Every state has a word; the summary's
  sentence names what is not healthy, worst first.
- **Hairline grids.** Each tile draws its right and bottom line (`box-shadow: inset -1px -1px`); the
  grid runs 1 pixel past its pane on the right and at the bottom, and the pane (`overflow: clip`, which
  keeps the sticky bars working) cuts the outermost lines, so no edge is doubled. The counts of the
  summary do the same.
- **Hover and focus.** Under the pointer a tile lifts to `--raised` and a glyph's frame inverts; a logo's
  frame only darkens its border, because a logo drawn for one scheme can vanish on the other's ground.
  A bookmark row inverts, a logo in it keeping a square of the page's ground. Keyboard focus is a 2-pixel
  `--accent` outline inside the element, `--accent-invert` where the ground is inverted.
- **The link's name begins with the app's.** In a tile the name comes first in the markup and the CSS
  grid places the icon, the status, the description and the address; the badges come last, over the
  icon's frame. In a bookmark row the number is `aria-hidden`.
- **Icons** ([item-icon.ts](../../frontend/src/app/shared/item-icon/item-icon.ts)): `tint` is a CSS
  mask filled with the current colour, so a glyph follows the scheme and the inversion; a logo is an
  `<img>`, its `src_dark` in the dark scheme. 28 pixels in a tile's 48-pixel frame, 20 in a bookmark
  row. An SVG is drawn only as a mask or an image, where its scripts never run
  ([docs/security/served-icons.md](../security/served-icons.md)).
- **Badges** ([badge.ts](../../frontend/src/app/shared/badge/badge.ts)): square, 18 pixels high, at
  least as wide, a third outside the icon frame's corner, cut from it by a 2-pixel ring in the tile's
  colour (`--badge-ring`); filled with the tone, the glyph or text in the tone's ink; `role="img"` with
  the label as its name and tooltip. On a bookmark row a 16-pixel chip after the name.
- **Section bars** carry a number (`01 /`), hidden from screen readers, and the section's `h2`; they
  stay under the top bar (`position: sticky; top: var(--nav-height)`) while their section scrolls.
  **Group heads** ([group-head.ts](../../frontend/src/app/shared/group-head/group-head.ts)) are an `h3`
  of the name and the count.
- **The horizon** ([horizon.ts](../../frontend/src/app/features/dashboard/horizon.ts)) only where there
  is no data: no app and no bookmark (naming what the discovery reads, so a selector that matches
  nothing is told from an empty cluster), a backend that is starting or cannot be reached.
- **Links** carry the app's URL as Angular binds it — a `javascript:` URL would be marked unsafe, a
  second line behind the backend's http-or-https rule; a link that opens a new tab carries
  `rel="noopener noreferrer"`.
- **Width**: grids of `minmax(260px, 1fr)`; one column at phone width with a 16-pixel gutter (`--pad`)
  and no horizontal scroll (the end-to-end suite measures it). The crosshair ticks, which sit 6 pixels
  outside the frame, are drawn only from 1,300 pixels.
- **Faces** (`angular.json` styles): `@fontsource-variable/inter`, `@fontsource-variable/archivo`,
  `@fontsource-variable/geist-mono` and `@fontsource/instrument-serif`'s italic; served from the page's
  origin, as its policy requires.

## Tokens and contrast

[tokens.spec.ts](../../frontend/src/app/design/tokens.spec.ts) reads `tokens.css` and holds, in both
schemes: every text token — the status colours included, since they are words and counts — to 4.5:1
on every surface (`--bg`, `--raised`, `--down-tint`); every shape (the accent, the status colours,
`--line-strong`, the inks) to 3:1; amber to 4.5:1 on the stale notice's tint; the inverted text to
4.5:1 and `--accent-invert` to 3:1 on `--invert-bg`; each badge's ink to 4.5:1 on its tone, and the
tone to 3:1 on every surface; and no violet anywhere. A new colour is a `light-dark()` pair there,
with its case in the test.

## Adding a component

1. A standalone component with `ChangeDetectionStrategy.OnPush`, signal inputs, its CSS in the tokens
   only, under `shared/` when more than one feature uses it, else under its feature.
2. A spec beside it that renders it with `TestBed.createComponent` and its inputs, and checks what a
   person and a screen reader get; fixtures from `src/testing/`.
3. A new colour in `tokens.css` with its contrast case; motion through `--motion`, which is 0 under
   `prefers-reduced-motion`; corners square.
