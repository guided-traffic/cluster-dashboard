# ADR 0009: Remote Icons Are Fetched by the Backend Under Cogpit's Rules

## Status

Accepted 2026-10-26 (the plan's §7.6). **Built** 2026-10-26 (phase 2, step 4) in
`backend/internal/icons/fetch.go` and `remote.go`; amended the same day with D4. Verified by the unit
tests: the address matrix against TLS servers on loopback with the lookup replaced, the redirects, the
size and time bounds, and the table — its bound, the hour, the readiness.

## Decision

**D1 — At start and on discovery**, never by the browser; failures retried after an hour; no periodic
refresh.

**D2 — Cogpit's `IconFetcher` and `CheckIcon`, ported**: https only (http only for an allowed host), no
credentials, cookies or referrer, at most 256 KiB in 20 seconds and five redirects, non-public
addresses refused at the address dialled, raster re-encoded, SVG only as one self-contained document.

**D3 — Annotation URLs** are fetched only while `icons.remote.fromAnnotations` is on
([docs/security/remote-icons.md](../security/remote-icons.md) names the risk).

**D4 — Where Dash differs from Cogpit's fetcher**: plain http for a host of `allowedHosts`, and
`allowPrivateNetworks`, both named as escape hatches; one transport for every download, the allowed
hosts being the configuration's; no proxy, so the address check holds; GIF and WebP decoded too; the
log shows a URL without its user and query. A URL is registered when it is read — the values' at
start, an annotation's when the discovery reads the Ingress — and `/readyz` waits for the downloads
of the first list.

## References

- [docs/planning/project-plan.md](../planning/project-plan.md) §7.6, §12
