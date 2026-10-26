# Served icons

Case 3 of the [plan](../planning/project-plan.md) §12: an SVG can carry script that runs when the file
is opened as a document — and every icon Dash serves is served from the dashboard's own origin.

| Source | Who controls the bytes | Checked |
|---|---|---|
| A bundled catalogue | The catalogue's maintainers and contributors (community submissions); pinned per release or commit ([supply-chain.md](supply-chain.md)) | At image build: every SVG through `CheckSVG`; one that fails is left out |
| An icon ConfigMap | Whoever writes `icons.configMap.files` or an existing ConfigMap of the release namespace | At start: `CheckIcon`; one that fails is left out with a warning |
| A URL | Whoever controls the remote host | At download: `CheckIcon` ([remote-icons.md](remote-icons.md)) |

## The check

[backend/internal/icons/check.go](../../backend/internal/icons/check.go), a port of Cogpit's
`CheckIcon`:

- **Raster images** (PNG, JPEG, GIF, WebP, told by their magic bytes) are decoded under bounds —
  the header first: at most 8192 pixels a side and 16 megapixels, so a small file cannot expand
  into a huge image — and **encoded again as a PNG** of at most 256 pixels a side. What reaches the
  browser is a PNG Dash wrote.
- **SVG** is kept as text only as one `<svg>` document of at most 256 KiB, well-formed XML of
  bounded depth (64) and tokens (20,000), without a DOCTYPE or another directive (no entities), a
  processing instruction other than the XML declaration, `script`, `foreignObject`, `iframe`,
  `object`, `embed`, `audio`, `video`, `animate`, `set`, `animateMotion`, `animateTransform` or
  `handler`, an event-handler attribute (`on…`), an `@import`, or a reference outside itself — an
  `href` other than `#fragment` or an embedded raster image (`data:image/png;base64,…` and its
  JPEG, GIF and WebP forms), a `url(…)` other than `url(#fragment)`.

Where Dash differs from Cogpit: GIF and WebP are decoded; an embedded raster image is accepted, as
many catalogue logos carry one and raster data runs nothing — an embedded SVG is not; `@import`,
which Cogpit's check let through when it named a string rather than `url(…)`, is refused.

## The serving

Every icon is answered with `Content-Type` from the check (`image/svg+xml` or `image/png`, never the
source's), `X-Content-Type-Options: nosniff` and
`Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; sandbox`: an SVG opened on
its own loads nothing and runs nothing, and the sandbox gives it an opaque origin. The page draws an
icon only as `<img>` or as a CSS mask ([item-icon.ts](../../frontend/src/app/shared/item-icon/item-icon.ts),
[badge.ts](../../frontend/src/app/shared/badge/badge.ts)), where an SVG's script never runs.

## What this does not cover

### H-5 A catalogue logo is trusted to be the app's

The check makes an icon inert, not truthful: a community submission could put a misleading picture
under an app's name, and auto-match would show it. The pins make every change a reviewed pull
request; the content itself is not reviewed.
