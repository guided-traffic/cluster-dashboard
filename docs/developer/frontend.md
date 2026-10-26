# Frontend

The page: Angular 22.2, standalone, zoneless, signals, no component library
([ADR 0005](../adr/0005-cogpits-dark-cockpit-on-the-web-without-a-component-library.md)). It reads one
answer — `GET /api/v1/dashboard` — and draws it in Cogpit's Dark Cockpit; every decision about an
item (its icon, its fallback, its badges, what is hidden) was made by the backend.

## The tree

| Path | Responsibility |
|---|---|
| [src/app/app.ts](../../frontend/src/app/app.ts) | The shell: the top bar — `ui.title` (also the document's title), the greeting and the date, the backend's version, the theme control — over the dashboard |
| [src/app/core/dashboard.service.ts](../../frontend/src/app/core/dashboard.service.ts) | Asks for the dashboard on start, every `ui.refresh_seconds`, at once when the page becomes visible again, never while it is hidden; keeps the last answer when a refresh fails |
| [src/app/core/theme.service.ts](../../frontend/src/app/core/theme.service.ts) | The colour scheme: the stored choice, else `ui.default_theme` once the dashboard has answered, else `system` |
| [src/app/core/clock.ts](../../frontend/src/app/core/clock.ts) | The greeting's time of day and the date, to the minute |
| [src/app/features/dashboard/](../../frontend/src/app/features/dashboard/dashboard.ts) | The page: the summary, the app groups, the bookmarks, the horizon where there is nothing to show |
| [src/app/shared/](../../frontend/src/app/shared) | `item-icon` (a glyph as a mask, a logo as an image), `badge` (a pill at a corner or a chip), `bracket` (a group head), `theme-control` |
| [src/app/design/tokens.css](../../frontend/src/app/design/tokens.css) | Every colour as a `light-dark()` pair: surfaces, text, lights, status tints and words, badge tones |
| [src/testing/](../../frontend/src/testing/dashboard.ts) | Fixtures of the dashboard answer, for the specs only |
| `src/app/api/` | The client ng-openapi-gen writes (`make frontend-generate`), never edited, never formatted |

## The page's rules

- **Quiet when healthy.** A healthy tile draws no colour. Degraded: a 3-pixel amber spine at the left
  edge and the pill `2 of 3 ready`; down: a coral spine, a 6 % coral tint (3.5 % in light) and the pill
  `down`. Every state has a word; the summary's legend names only what is not healthy.
- **One light per region.** Hover and keyboard focus draw Cogpit's glow frame — four box-shadow rings
  of falling alpha in glow blue — and light the glyph; nothing else glows.
- **Icons** ([item-icon.ts](../../frontend/src/app/shared/item-icon/item-icon.ts)): `tint` is a CSS
  mask filled with the current colour, so a glyph follows the scheme; a logo is an `<img>`, its
  `src_dark` in the dark scheme. An SVG is drawn only as a mask or an image, where its scripts never
  run ([docs/security/served-icons.md](../security/served-icons.md)).
- **Badges** ([badge.ts](../../frontend/src/app/shared/badge/badge.ts)): 18 pixels high, at least as
  wide, a third outside the icon's corner, cut from it by a 2-pixel ring in the tile's colour
  (`--badge-ring`); filled with the tone, the glyph or text in the tone's ink; `role="img"` with the
  label as its name and tooltip. On a bookmark row a chip after the name.
- **Brackets** ([bracket.ts](../../frontend/src/app/shared/bracket/bracket.ts)): the label with a middle
  dot and the count over a hairline at 55 % with 6-pixel end ticks; cyan for the cluster's groups and
  the summary, muted for the bookmarks.
- **The horizon** ([horizon.ts](../../frontend/src/app/features/dashboard/horizon.ts)) only where there
  is no data: no app and no bookmark (naming what the discovery reads, so a selector that matches
  nothing is told from an empty cluster), a backend that is starting or cannot be reached.
- **Links** carry the app's URL as Angular binds it — a `javascript:` URL would be marked unsafe, a
  second line behind the backend's http-or-https rule; a link that opens a new tab carries
  `rel="noopener noreferrer"`.
- **Width**: grids of `minmax(240px, 1fr)`; one column at phone width with a 16-pixel gutter and no
  horizontal scroll (the end-to-end suite measures it).

## Tokens and contrast

[tokens.spec.ts](../../frontend/src/app/design/tokens.spec.ts) reads `tokens.css` and holds, in both
schemes: every text token to 4.5:1 on every surface; every light and status colour to 3:1; each status
word to 4.5:1 on its tint; each badge's ink to 4.5:1 on its tone, and the tone to 3:1 on every surface;
and no violet anywhere. A new colour is a `light-dark()` pair there, with its case in the test.

## Adding a component

1. A standalone component with `ChangeDetectionStrategy.OnPush`, signal inputs, its CSS in the tokens
   only, under `shared/` when more than one feature uses it, else under its feature.
2. A spec beside it that renders it with `TestBed.createComponent` and its inputs, and checks what a
   person and a screen reader get; fixtures from `src/testing/`.
3. A new colour in `tokens.css` with its contrast case; motion through `--motion`, which is 0 under
   `prefers-reduced-motion`.
