# ADR 0007: Discovery Has Hajimari's Semantics, Fixed Where Hajimari Is Wrong, and Watches Instead of Polling

## Status

Accepted 2026-10-26 (the plan's §6, from Hajimari's source at `b07024d`). **Not built** — phase 2.

## Context

Hajimari has been idle since 2023-05-25. Its source shows what discovery should keep and what it gets
wrong: the URL from `tls[0].hosts[0]` breaks on wildcards and reordered TLS blocks; the readiness ratio
is rounded to 0 or 1; every Ingress and EndpointSlice is re-listed every 60 seconds.

## Decision

**D1 — The keys of Hajimari**: `enable`, `appName`, `group`, `icon`, `url`, `info`, `targetBlank`,
`instance`, `statusCheckEnabled`, with Hajimari's meaning, under Dash's prefix
([ADR 0010](0010-annotations-are-read-under-dash-gtrfc-com-only.md)).

**D2 — Selection**: `defaultEnable`, `instanceName`, `namespaceSelector` (`any`, `matchNames`,
`labelSelector`, united). Where Hajimari lets `any: true` silently win over names and a label
selector, Dash refuses the combination at install and at start; `any: false` with neither selects
nothing and is refused while the discovery is enabled (built with the configuration, phase 2
step 1).

**D3 — The URL**: the first rule with a concrete host; `https` when that host is in any TLS block; its
first `Prefix` or `Exact` path without regular-expression characters, else `/`; `url` only with `http`
or `https`.

**D4 — Status**: ready endpoints over all endpoints of the referenced Services, each pod once.

**D5 — Informers** on Ingresses and EndpointSlices (Namespaces for a label selector), trimmed; the API
reads the cache.

## References

- [docs/planning/project-plan.md](../planning/project-plan.md) §6
