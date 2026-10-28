# ADR 0013: The Environment Under Cogpit's Light Strip, No Counts of States

## Status

Accepted 2026-10-28 (the owner: the summary's sentence, its squares and its counts can go — Dash is
a dashboard, not a small monitoring; the environment goes on top, "Good morning, welcome to
<ENVIRONMENT>"; Cogpit's light strip over the greeting, its colour and animation set through the
chart). Amends [ADR 0012](0012-a-technical-editorial-look-the-apps-beside-the-bookmarks.md) D2.
**Built** 2026-10-28. Verified by the unit tests of both modules and the chart, and by `make e2e`
in Chromium and WebKit: the greeting, the scanner's light and its colour in both schemes, the light
standing still for reduced motion, the grain at the display's density.

## Context

ADR 0012's overview carried a sentence naming the apps that are not healthy, a square per app and a
row of counts by state. The owner judged them a monitoring's, not a dashboard's: a tile says its own
state, and that is enough. What the top of the page should say instead is where one is — which
cluster, which environment.

Cogpit marks a cluster the same way: a light strip under a view's header, lit in the cluster's
accent colour, still or moving (Cogpit's D-122 in its `docs/developer/decisions.md`; its maths in
`internal/ui/fyneui/lightstrip_model.go`). A Dash serves
one cluster, so its strip has one colour, set at install.

## Decision

**D1 — No counts of states.** The overview has no summary: no sentence, no squares, no counts.
`ui.showSummary` is gone, and so is the API's `summary`, which nothing else read. A tile keeps its
status word and its line (ADR 0012 D3); the apps' bar keeps its count of apps and groups.

**D2 — The greeting welcomes to the environment.** *Good morning, welcome to production.*, the time
of day in the serif's italic: `ui.greeting.environment` replaces `ui.greeting.name`. Without an
environment the greeting is *Good morning.*; with `ui.greeting.enabled` off, the title as before.

**D3 — Cogpit's light strip over the greeting.** A strip of frosted glass 4 pixels high, the
frame's width, between the overview's bar and the greeting
([light-strip.ts](../../frontend/src/app/features/dashboard/light-strip.ts)). Cogpit's look, ported:

- **Glass**: the page's ground mixed as Cogpit mixes it (toward white by 5.88 % in dark, toward
  `#7f8da3` by 9.8 % in light) — `--glass`, `#202124` and `#e7e7e7`; Cogpit's grain over it and over
  the light, its own 256 × 64 texture from the same generator and seed, a speck per device pixel,
  drawn on a canvas.
- **Colour** (`ui.lightStrip.color`): empty for the glass alone, the default; one of `cyan`, `blue`,
  `magenta`, `red`, `orange`, `yellow`, `green` — Cogpit's accents in Cogpit's shade of each scheme,
  each held to 3:1 on the glass; or `#rrggbb`, drawn as given in both schemes.
- **Animation** (`ui.lightStrip.animation`): `static`, the default; `breathing` (down to 48 % and
  back every 4.6 s); `scanner` (a light half the strip wide, entering from beyond the left edge,
  then swinging between its ends every 6 s); `flow` (a light 46 % of the strip long with a soft
  front a quarter of that, crossing every 5 s, the next setting off as the head goes under). Every
  sweep eased as Cogpit eases it, in CSS animations of opacity and transform.
- The light fades in from the glass over 350 ms and starts from its beginning whenever its colour or
  animation changes. Where the browser asks for reduced motion it stands still at full strength over
  the whole strip.
- Decorative (`aria-hidden`): the greeting names the environment in words.

**D4 — Violet is no colour of the strip.** ADR 0005 leaves violet to Cogpit's agent, and the
tokens hold no violet; `#rrggbb` reaches it where an installation wants it.

## Consequences

- A breaking change of the values: a file that sets `ui.greeting.name` or `ui.showSummary` is
  refused at install by `values.schema.json`, and by the backend when it reaches the file.
- A breaking change of `GET /api/v1/dashboard`: `summary`, `ui.show_summary` and `ui.greeting.name`
  are gone; `ui.greeting.environment` and `ui.light_strip` are new.
- What the strip does not port from Cogpit: a colour per cluster (one Dash, one cluster); the old
  light fading out through the glass before the new one comes in — it goes at once, since the
  colour changes only with an upgrade; the spread and the crossfade of one static colour into
  another; standing still while the window is in the background or on battery; the derived legible
  shade of a free colour.
- The load of an animated strip on a client was not measured. Cogpit measured its own (a fifth of a
  core, Fyne repainting the whole window); a browser animates opacity and transform without
  re-laying the page out, which is why the strip is drawn so, not a measurement.
- The page's initial bundle: 199.97 kB raw (`make frontend-build`), against 200.35 kB with the
  summary. Angular's `afterNextRender` and `viewChild` would have added about 12 kB; the strip uses
  `ngAfterViewInit` and the host element instead.

## Alternatives considered

| Alternative | Why not |
|---|---|
| Keep the summary behind `ui.showSummary`, off by default | A switch for what the owner does not want on a dashboard; the API would carry counts for nobody |
| Keep the name beside the environment: *Good morning, Hans, welcome to production.* | The owner's line names the environment only |
| The strip under the top bar, over the whole page | The owner asked for it over the greeting; under the top bar it would sit against the overview's sticky bar, the place where Cogpit's strip clashed with the active tab's underline |
| Dash's badge tones as the strip's colours | The owner asked for Cogpit's strip; Cogpit's names and shades let one choice mean the same colour in both tools |
| The grain as an SVG turbulence or a picture file | Not Cogpit's specks, or not at a speck per device pixel; a canvas needs no change of the frontend's policy |
| Animating the light from script, frame by frame | CSS runs it without a timer in the page and stands it still for reduced motion by a media query |

## References

- `frontend/src/app/features/dashboard/light-strip.ts`, `light-strip.css`, `light-strip.spec.ts`,
  `frontend/src/app/design/tokens.css` (`--glass`, `--strip-*`), `frontend/e2e/page.spec.ts`,
  `backend/internal/config/` (`ui.lightStrip`, `ui.greeting.environment`),
  `backend/api/components/schemas.yaml` (`LightStrip`), `deploy/helm/dash/values.yaml`
- Cogpit D-122 and D-80: `docs/developer/decisions.md`; `internal/ui/fyneui/lightstrip_model.go`,
  `identitycolor.go`, `theme.go` (`identityColors`)
