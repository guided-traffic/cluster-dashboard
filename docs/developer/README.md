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
5. **Phase 1 is a skeleton**: the dashboard itself — discovery, icons, badges, tiles — is phase 2 of
   the [plan](../planning/project-plan.md).

## Pages

| Page | Topic |
|---|---|
| [repository-layout.md](repository-layout.md) | The tree, file by file where it matters |
| [build-test-lint.md](build-test-lint.md) | Every make target, local development |
| [testing.md](testing.md) | The test tiers, what each proves, their fixtures |
| [ci-and-release.md](ci-and-release.md) | The workflows, the gates, the release, Renovate |
| [conventions.md](conventions.md) | Commits, errors, logs, generated code, Kubernetes safety |

Decisions: [docs/adr/](../adr/README.md). Security architecture: [docs/security/](../security/README.md).

## The request path

```text
browser ─┬─ https://<host>/        → Ingress <fullname>-frontend → Service → frontend/server (bundle in memory)
         └─ https://<host>/api/... → Ingress <fullname>-backend  → Service → backend (dash serve)
                                                                      └── Kubernetes API (version probe)
```

## Start-up of the backend

`dash serve` loads the configuration ([ADR 0004](../adr/0004-configuration-is-three-variables-and-one-validated-file.md)),
builds the Kubernetes client (the pod's ServiceAccount in a cluster, client-go's loading rules
elsewhere), starts the probe that asks the API server for its version until it answers once, and
listens. `/healthz` answers at once; `/readyz` answers `503 not_ready` until the probe has its answer.
`SIGTERM` gives the requests in flight 15 seconds.
