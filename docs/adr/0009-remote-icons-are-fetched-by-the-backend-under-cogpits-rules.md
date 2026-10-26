# ADR 0009: Remote Icons Are Fetched by the Backend Under Cogpit's Rules

## Status

Accepted 2026-10-26 (the plan's §7.6). **Not built** — phase 2.

## Decision

**D1 — At start and on discovery**, never by the browser; failures retried after an hour; no periodic
refresh.

**D2 — Cogpit's `IconFetcher` and `CheckIcon`, ported**: https only (http only for an allowed host), no
credentials, cookies or referrer, at most 256 KiB in 20 seconds and five redirects, non-public
addresses refused at the address dialled, raster re-encoded, SVG only as one self-contained document.

**D3 — Annotation URLs** are fetched only while `icons.remote.fromAnnotations` is on
([docs/security/](../security/README.md) names the risk).

## References

- [docs/planning/project-plan.md](../planning/project-plan.md) §7.6, §12
