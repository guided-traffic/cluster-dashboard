# ADR 0001: Two Images, Two Ingress Objects, One Chart — Distroless, a Spec-First API, Ready Once the API Server Answered

## Status

Accepted 2026-10-26 (the owner's brief and the project plan, `docs/planning/project-plan.md` §5).
**Built and verified** on 2026-10-26: both images build and hold no shell (`make docker-build
docker-no-shell`); the chart renders both Ingress objects (`make helm-unittest`); in a kind cluster
behind haproxy-ingress 0.16.2 with cert-manager v1.21.3, the UI answered through the frontend's
object and `/api/v1/version` through the backend's, over TLS, in Chromium and WebKit (`make e2e`,
24 tests).

## Context

The owner fixed the cut: a Go backend in `backend/`, an Angular frontend in `frontend/`, two
Ingresses in the cluster and no nginx that proxies between them, and distroless containers for the
smallest attack surface. cowork, the sibling project, went from an nginx proxy to Ingress path
routing on 2026-10-04 (its ADR 0001 D3) because the proxy was an extra hop behind the reverse proxy
the Ingress already is.

## Decision

**D1 — Two images, one chart.** `guidedtraffic/dash-backend` (the API) and
`guidedtraffic/dash-frontend` (the UI), each a Deployment and a Service of the chart
`deploy/helm/dash`, released under one version.

**D2 — Two Ingress objects, the controller as the one hop.** `<fullname>-frontend` routes `/` and
`<fullname>-backend` routes `/api/` (both `Prefix`), on the same hosts with the same TLS block.
`ingress.annotations` go on both, `ingress.frontend.annotations` and `ingress.backend.annotations`
on their own object. The paths are the chart's, not values. The browser sees one origin, so there
is no CORS. Both objects carry `dash.gtrfc.com/enable: "false"`, so Dash does not list itself.

**D3 — Both images are distroless.** `gcr.io/distroless/static-debian13:nonroot`, pinned by digest:
no shell, no package manager, no libc. `hack/no-shell.sh` proves it for each image in CI. Both pods
run as 65532 with a read-only root file system and no writable volume, all capabilities dropped,
no privilege escalation, the `RuntimeDefault` seccomp profile; the frontend's pod mounts no
ServiceAccount token.

**D4 — The frontend's server is a Go file server, standard library only** (`frontend/server`, its
own module). It loads the bundle into memory at start and applies cowork's nginx rules: every
router path is `index.html` with `no-store`, hashed files are `immutable`, the shell's
Content-Security-Policy is on every UI response, `/healthz` is its own, and an `/api/` path that
reaches it is a `404` problem naming the misrouting. It never reaches the backend.

**D5 — The API is spec-first.** `backend/api/openapi.yaml` (OpenAPI 3.1) is the source; oapi-codegen
generates the strict server interface and the models, kin-openapi validates requests against the
document, ng-openapi-gen generates the frontend's client. Every error is an RFC 9457 problem body
with a stable `code` and the request id (`backend/internal/problem`).

**D6 — Ready once the API server answered.** `/readyz` turns ready when the Kubernetes API server
has answered a version request once since the start (`backend/internal/kube`); a later outage does
not take every replica out of its Service at once. `/healthz` says the process serves.

## Consequences

- Without an Ingress there is no one origin: a port-forward to the frontend alone serves the UI
  without its API; the chart's notes say so.
- An authentication annotation on the frontend's object alone leaves `/api/` open
  ([docs/security/exposure.md](../security/exposure.md)).
- A pod without a shell is debugged through an ephemeral container.
- The file server is code of ours with unit tests (`frontend/server/server_test.go`), where nginx's
  configuration could only be checked by running the image.

## Alternatives considered

- **One Ingress object with two paths** (cowork's form). Equal routing; loses per-object
  annotations. The owner asked for two objects.
- **nginx for the frontend.** No distroless nginx image is published with version tags — Chainguard's
  free `cgr.dev/chainguard/nginx` offers `latest` and `latest-dev` only (checked 2026-10-26) — and its
  configuration is tested only by running the image.
- **The UI embedded in the backend binary.** One image; contradicts the brief's two containers.

## References

- `deploy/helm/dash/templates/ingress.yaml`, `backend/Containerfile`, `frontend/Containerfile`,
  `frontend/server/`, `backend/api/`, `backend/internal/kube/kube.go`, `hack/no-shell.sh`, `hack/e2e.sh`
