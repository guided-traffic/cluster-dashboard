# ADR 0007: Discovery Has Hajimari's Semantics, Fixed Where Hajimari Is Wrong, and Watches Instead of Polling

## Status

Accepted 2026-10-26 (the plan's §6, from Hajimari's source at `b07024d`). **Built** 2026-10-26 (phase
2, step 2) in `backend/internal/discovery` and `backend/internal/dashboard`, with the chart's
`templates/rbac.yaml`; amended the same day with D4's reading of an empty slice and a terminating
endpoint, and D6 and D7. Verified by the unit tables, by the informers on a fake clientset, and by
the integration tier, which runs the discovery as the chart's ServiceAccount under the RBAC
`helm template` renders for each selector mode.

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

**D4 — Status**: ready endpoints over all endpoints of the referenced Services, each pod once; a
terminating endpoint is not counted, so a rollout does not light the tile. No EndpointSlice at all is
`unknown` (nothing to count); slices without a ready endpoint — a Deployment scaled to zero leaves
its slice empty — are `down`.

**D5 — Informers** on Ingresses and EndpointSlices (Namespaces for a label selector), trimmed; the API
reads the cache. An Ingress is read when it changes, its status counted when the page asks; ready
once every informer has listed.

**D6 — The RBAC follows the selector**: a ClusterRole for `any` and for a label selector (plus
Namespaces), a Role in each named namespace for names alone — the informers' layout. The objects are
named `<release namespace>-<fullname>`, so two releases of one name in two namespaces do not claim
the same ClusterRole.

**D7 — What the configuration hides is not served**: `ui.showApps`, `ui.showBookmarks`,
`ui.showAppInfo` and `ui.showAppStatus` off leave apps, bookmarks, descriptions and statuses out of
what the backend answers, not only out of the page.

## References

- [docs/planning/project-plan.md](../planning/project-plan.md) §6
- [docs/developer/discovery.md](../developer/discovery.md), [docs/security/privilege-footprint.md](../security/privilege-footprint.md)
