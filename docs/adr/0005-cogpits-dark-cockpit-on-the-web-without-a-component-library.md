# ADR 0005: Cogpit's Dark Cockpit on the Web, No Component Library, and a Remembered Colour Scheme

## Status

Accepted 2026-10-26 (the owner: "the look is Cogpit's"). **Built** for the shell: the tokens of both
schemes, the theme service, the segmented control, `public/theme-init.js`; verified by the unit tests
(100 % of the frontend's statements) and in Chromium and WebKit by `make e2e`. The dashboard's tiles,
groups, badges and summary arrive with phase 2.

## Context

Cogpit's design language (its `docs/developer/design-language.md`) is a dark cockpit: when every system
is normal no light is on; only a deviation lights up. Hajimari is the reference for discovery, never
for the look. The UI has no form, no table and no dialog — every setting is the chart's, there is no
login.

## Decision

**D1 — The tokens are Cogpit's palettes**, as CSS custom properties with `light-dark()` pairs
(`frontend/src/app/design/tokens.css`). Glow blue means you, cyan the cluster; violet stays the agent's
and is not used; status colours only for deviations.

**D2 — No component library.** Angular, the CDK where a behaviour needs it, components of our own.
PrimeNG would bring a license key to keep out of the repository and to pass to CI, for widgets the UI
does not have.

**D3 — The colour scheme is a preference of the browser**: `system`, `light` or `dark`, stored under
`dash.theme` in `localStorage`; `system` follows `prefers-color-scheme` live; storage that throws reads
as `system`. `public/theme-init.js`, loaded from the origin in `<head>`, applies the stored choice
before the first paint, because the CSP refuses inline scripts.

**D4 — The UI's own glyphs are Material Symbols, inlined** in the bundle, so the shell draws while the
backend is down. The font is Inter, self-hosted from npm; nothing loads from a CDN.

## Consequences

- Every component is styled for both schemes from the start; the contrast of the token pairs is a test.

## References

- `frontend/src/app/design/`, `frontend/src/app/core/theme.service.ts`,
  `frontend/src/app/shared/theme-control/`, `frontend/public/theme-init.js`
