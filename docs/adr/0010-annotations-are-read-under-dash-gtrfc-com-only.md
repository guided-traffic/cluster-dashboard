# ADR 0010: Annotations Are Read Under `dash.gtrfc.com/` Only

## Status

Accepted 2026-10-26, decided by the owner. **Built**: the chart's own Ingress objects carry
`dash.gtrfc.com/enable: "false"`; the discovery reads the keys under this prefix only
(`backend/internal/discovery/annotations.go`), and `TestHajimarisAnnotationsChangeNothing` proves a
`hajimari.io/*` set changes nothing (phase 2, step 2). Amended 2026-10-28: moving the owner's
clusters is not this project's work, so the first consequence names any manifest.

## Context

The organisation names its keys `<product>.gtrfc.com` (`dex.gtrfc.com`, `vko.gtrfc.com`,
`stackit-bucket.gtrfc.com`). About thirty of the owner's manifests carry `hajimari.io/*`.

## Decision

**D1 — `dash.gtrfc.com/<key>` only**; nothing under `hajimari.io/` is read.

**D2 — The keys keep Hajimari's names**, so a manifest migrates by replacing the prefix.

## Consequences

- A manifest moving from Hajimari gets the new keys beside the old ones while both dashboards run
  (the README's move from Hajimari).
- A test proves that a `hajimari.io/*` annotation changes nothing.

## Alternatives considered

- Dash's prefix with `hajimari.io/*` read as a fallback — recommended, rejected by the owner.
- `hajimari.io/*` alone — another project's name space.
