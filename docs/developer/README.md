# Developer documentation

For contributors and agents. Read [AGENTS.md](../../AGENTS.md) for the rules in short, then this page.

## What has to be in your head first

1. **Two images, two Ingress objects** ([ADR 0001](../adr/0001-two-images-two-ingress-objects-one-chart.md)):
   the Go backend serves `/api/` and the health endpoints, the frontend's Go file server serves the
   Angular bundle; the cluster's Ingress controller routes between them. Nothing proxies inside.
2. **Distroless**: neither image has a shell; CI proves it.
3. **Spec-first**: `backend/api/openapi.yaml` is the API; the Go server interface, the bundled
   document and the Angular client are generated from it and committed.
4. **`make` is the entry point**, locally and in CI; every tool is pinned once and built into `bin/`
   ([ADR 0006](../adr/0006-one-version-of-every-tool.md)).
5. **One setting, one name**: the chart renders its values `discovery`, `ui`, `apps`, `bookmarks`,
   `icons` and `badges` into the backend's file unchanged; the schema and the backend validate them
   ([configuration.md](configuration.md)).
6. **The cluster is the state**: the discovery watches Ingresses and EndpointSlices through trimmed
   informers and answers from their caches; nothing is stored ([discovery.md](discovery.md)).
7. **Every icon from the dashboard's origin**: five catalogues packed into the backend image from
   pins, ConfigMap and remote images checked and kept in memory; the backend decides every fallback
   ([icons.md](icons.md)).
8. **Phase 2 is in progress**: configuration, discovery, the dashboard API, icons and badges are
   built; the tiles follow, as the [plan](../planning/project-plan.md) lists them.

## Pages

| Page | Topic |
|---|---|
| [repository-layout.md](repository-layout.md) | The tree, file by file where it matters |
| [build-test-lint.md](build-test-lint.md) | Every make target, local development |
| [testing.md](testing.md) | The test tiers, what each proves, their fixtures |
| [ci-and-release.md](ci-and-release.md) | The workflows, the gates, the release, Renovate |
| [conventions.md](conventions.md) | Commits, errors, logs, generated code, Kubernetes safety |
| [configuration.md](configuration.md) | From the chart's values to the backend's `File`, validated twice; adding a key |
| [discovery.md](discovery.md) | Ingresses to apps: informers, selection, URL, status, RBAC per mode; the dashboard model; adding an annotation key |
| [icons.md](icons.md) | The packs and their pins, resolution and auto-match, ConfigMap and remote icons, serving; adding a catalogue |
| [badges.md](badges.md) | Definitions, rules, corners, the effective IngressClass; adding a match field |

Decisions: [docs/adr/](../adr/README.md). Security architecture: [docs/security/](../security/README.md).

## The request path

```text
browser ─┬─ https://<host>/        → Ingress <fullname>-frontend → Service → frontend/server (bundle in memory)
         └─ https://<host>/api/... → Ingress <fullname>-backend  → Service → backend (dash serve)
                                                                      │  /api/v1/dashboard: discovery cache
                                                                      │  + configuration → dashboard.Build
                                                                      └── Kubernetes API (version probe;
                                                                          list and watch: Ingresses,
                                                                          EndpointSlices, Namespaces)
```

## Start-up of the backend

`dash serve` loads the configuration and refuses to start on any problem with it
([ADR 0004](../adr/0004-configuration-is-three-variables-and-one-validated-file.md)),
builds the Kubernetes clients (the pod's ServiceAccount in a cluster, client-go's loading rules
elsewhere), starts the probe that asks the API server for its version until it answers once and —
unless `discovery.enabled` is off — the discovery's informers, and listens. `/healthz` answers at
once; `/readyz` answers `503 not_ready` until the probe has its answer and the informers have listed.
`SIGTERM` gives the requests in flight 15 seconds; `serve` returns once the probe and the informers
have stopped. client-go's own messages go through klog into the same JSON log.
