# ADR 0012: A Technical Editorial Look, the Apps Beside the Bookmarks

## Status

Accepted 2026-10-28 (the owner: the page did not look as smart as intended, a view split in two,
the direction of the support page of getartcraft.com; of a prototype's two splits, variant A).
Supersedes [ADR 0005](0005-cogpits-dark-cockpit-on-the-web-without-a-component-library.md) D1, D6
and the typeface of D4; its D2, D3, D5 and the glyphs of D4 stand. **Built** 2026-10-28 in
`frontend/`. Verified by the unit tests (every statement of the frontend, every token pair's
contrast in both schemes) and by `make e2e` in Chromium and WebKit, at desktop and phone width.

## Context

ADR 0005 drew the page in Cogpit's Dark Cockpit: plates with rounded corners, cyan brackets over the
groups, a glow frame under the pointer, the bookmarks below the apps. The owner wanted a smarter
page, the apps and the bookmarks side by side, and named the support page of getartcraft.com as the
direction. Its stylesheet was read for its language: neutral grounds (`#f2f1ee`, `#121316`),
hairlines instead of plates and shadows, the cells of a grid divided by 1-pixel lines, crosshair
ticks where a line meets the frame, upper-case mono labels, numbered section bars that stay under
the top bar, a display face with one word in a serif italic, inversion under the pointer, square
corners. The language is what Dash takes; its assets, logo, copy and layout it does not.

A prototype showed two splits: A, the overview above the apps beside the bookmarks; B, the overview
and the bookmarks in a column beside the apps. The owner chose A.

## Decision

**D1 — The tokens are neutral.** Grounds and inks without a hue; one accent, the keyboard's focus;
amber for degraded, coral for down, nothing for healthy; the badge tones of ADR 0011 keep their
names. Each colour is one `light-dark()` pair in `frontend/src/app/design/tokens.css`; every text
holds 4.5:1 and every shape 3:1 on every surface, and that is a test.

**D2 — The page.** A top bar of cells divided by hairlines: a mark and `ui.title`, the page's only
`h1`; the backend's version; the theme control as three square cells, the chosen one inverted.
Under it a frame of at most 1,280 pixels with hairline edges:

- `01 Overview`: the date and the time in its bar; the greeting (*Good evening, Hans.*, the time of
  day in Instrument Serif's italic), or the title while `ui.greeting` is off; with `ui.showSummary`
  a sentence naming the apps, their groups and what is not healthy, a square per app linking to its
  tile, and the counts in a row of hairline cells.
- The split: `02 Apps`, the groups as hairline grids of tiles under group heads, beside
  `03 Bookmarks`, rows numbered across their groups with the address under the name and the badges
  as chips. Each section's bar stays under the top bar while its section scrolls. At phone width the
  split wraps: the apps first, the bookmarks below.
- A footer naming what the discovery reads and how often the page asks.

**D3 — Status in words, colour only for deviations.** A healthy tile is quiet: its ready count
(`2/2`) in faint text. Degraded: a 2-pixel amber line along the tile's top edge and `2 of 3 ready`
in amber. Down: a coral line, a coral tint and `down`. Unknown: nothing.

**D4 — Interaction without glow.** Under the pointer a tile lifts one step and the frame of a glyph
inverts — a logo keeps its ground, a logo drawn for one scheme would vanish on the other's; a
bookmark row inverts. The keyboard's focus is a 2-pixel accent outline inside the element,
`--accent-invert` on an inverted ground.

**D5 — Four typefaces, self-hosted.** Archivo for the display line and the names, Instrument Serif's
italic for the greeting's time of day only, Inter for text, Geist Mono for labels, addresses and
counts; each from its Fontsource package on npm, nothing from a font CDN — the frontend's policy
allows fonts from its own origin only (`font-src 'self' data:`).

**D6 — Square corners**, the badges included. The horizon stays the empty state of ADR 0005, its
grid floor in hairlines, without the stage light.

## Consequences

- ADR 0005 D1, D6 and its typeface are superseded and marked there; the shell, the theme preference
  and its control's radio group are unchanged.
- The bracket head is now `shared/group-head`; the status pill a word at the tile's far end (`.status`);
  the summary a sentence and a row of counts.
- A healthy tile shows its ready count: quiet, but no longer silent.
- The faces are separate files outside the initial bundle (`make frontend-build`: initial total
  200.35 kB raw, no budget warning). That a browser fetches only the subsets a page uses, by each
  face's `unicode-range`, is how the faces are declared; the requests were not measured.

## Alternatives considered

| Alternative | Why not |
|---|---|
| Variant B: the overview and the bookmarks in a column beside the apps | More apps without scrolling — about nine of thirteen against about three at 1,440 × 900, estimated from the prototype's measures, not measured; the owner chose A |
| Keep Cogpit's look | The owner asked for another |
| A single light/dark toggle, as the prototype drew it | The three-way preference of ADR 0005 D3, `system` included, stays |
| The faces from Google Fonts | The policy allows fonts from the page's origin only, and no third party should see who opens the page |

## References

- `frontend/src/app/design/tokens.css`, `frontend/src/app/design/tokens.spec.ts`,
  `frontend/src/app/app.html`, `frontend/src/app/features/dashboard/`,
  `frontend/src/app/shared/group-head/`, `frontend/angular.json` (the faces),
  [docs/developer/frontend.md](../developer/frontend.md)
