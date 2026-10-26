# Dash

[![Build Status](https://github.com/guided-traffic/dash/actions/workflows/release.yml/badge.svg)](https://github.com/guided-traffic/dash/actions)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/guided-traffic/dash/main/.github/badges/coverage.json)](https://github.com/guided-traffic/dash)
[![Go Report Card](https://goreportcard.com/badge/github.com/guided-traffic/dash)](https://goreportcard.com/report/github.com/guided-traffic/dash)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**A start page for a Kubernetes cluster that finds its apps through their Ingresses**, in the look of
[Cogpit](https://github.com/guided-traffic/cogpit): calm when every app is healthy, lit where one is
not. Installed by one Helm chart, configured entirely through it, no login.

> **Phase 1 of 3.** What exists is the walking skeleton: both images, the chart with its two Ingress
> objects, the pipeline and the shell with its colour scheme; no release is published yet. The dashboard itself — discovery,
> icons, bookmarks, badges — is phase 2 of the [project plan](docs/planning/project-plan.md).

```mermaid
flowchart LR
  b([Browser]) --> f["Ingress dash-frontend, path /"] --> fs["frontend: Go file server, distroless"]
  b --> a["Ingress dash-backend, path /api/"] --> be["backend: Go API, distroless"]
  be --> k[(Kubernetes API)]
```

## ✨ Key Features

- 🧭 One origin, two Ingress objects: the UI on `/`, the API on `/api/`, the controller the only hop.
- 🛡️ Distroless images, non-root, read-only, no shell — proven in CI for each image.
- 🌗 Light, dark or the system's scheme, remembered by the browser, applied before the first paint.
- 📜 A spec-first API: OpenAPI 3.1, validated requests, RFC 9457 problem bodies.
- ⚙️ Every setting through the chart, refused at install when wrong (`values.schema.json`).

## 📛 Naming conventions

| What | Name |
|---|---|
| Images | `guidedtraffic/dash-backend`, `guidedtraffic/dash-frontend` |
| Chart, repository | `dash`, `https://guided-traffic.github.io/dash/` |
| Resources | `<fullname>-backend` (Deployment, Service, Ingress, ServiceAccount), `<fullname>-frontend` (Deployment, Service, Ingress), `<fullname>-config` (ConfigMap) |
| Annotations | `dash.gtrfc.com/<key>` — read by the discovery from phase 2; the chart puts `dash.gtrfc.com/enable: "false"` on its own Ingress objects |
| Backend variables | `DASH_CONFIG`, `DASH_LISTEN_ADDR`, `DASH_LOG_LEVEL` |
| Configuration file | `/etc/dash/config.yaml` |
| Browser storage | `dash.theme` |
| API | `/api/v1/` |

## 📚 Documentation

| Where | What |
|---|---|
| [docs/developer/](docs/developer/README.md) | How the code works, building, testing, CI, conventions |
| [docs/security/](docs/security/README.md) | Trust boundaries, privilege footprint, exposure without a login |
| [docs/adr/](docs/adr/README.md) | Every decision and what it rejected |
| [docs/planning/project-plan.md](docs/planning/project-plan.md) | The phases still to come |
| [SECURITY.md](SECURITY.md) | Reporting a vulnerability |

## 🚀 TL;DR

Once the first release is published:

```bash
helm repo add dash https://guided-traffic.github.io/dash/
helm repo update
helm install dash dash/dash --namespace dash --create-namespace \
  --set ingress.enabled=true \
  --set ingress.className=haproxy \
  --set 'ingress.hosts[0]=dash.example.com'
```

Verify: `https://dash.example.com/` shows the shell with the backend's version beside the title, and
`https://dash.example.com/api/v1/version` answers with JSON.

> **Protect it.** Dash has no login. Put it on an internal Ingress class, or set authentication
> annotations under `ingress.annotations` — **not** under `ingress.frontend.annotations` alone, which
> would leave `/api/` open ([docs/security/exposure.md](docs/security/exposure.md)).

<details>
<summary>Upgrade and uninstall</summary>

```bash
helm repo update && helm upgrade dash dash/dash --namespace dash --reuse-values
helm uninstall dash --namespace dash
```

</details>

## 📖 Full reference

### Helm values

<details>
<summary>Every value with its default</summary>

```yaml
nameOverride: ""          # default
fullnameOverride: ""      # default
imagePullSecrets: []      # default; example: [{name: regcred}]

backend:
  image:
    repository: guidedtraffic/dash-backend   # default
    tag: ""                                  # default: the chart's appVersion
    pullPolicy: IfNotPresent                 # default; Always | IfNotPresent | Never
  replicas: 1                                # default
  logLevel: info                             # default; debug | info | warn | error
  resources:                                 # default
    requests: { cpu: 10m, memory: 64Mi }
    limits: { memory: 256Mi }                # also the Go runtime's GOMEMLIMIT
  podLabels: {}                              # default; example: { gnp/k8s-api-access: "" }
  podAnnotations: {}                         # default
  nodeSelector: {}                           # default
  tolerations: []                            # default
  affinity: {}                               # default

frontend:
  image:
    repository: guidedtraffic/dash-frontend  # default
    tag: ""                                  # default: the chart's appVersion
    pullPolicy: IfNotPresent                 # default
  replicas: 1                                # default
  resources:                                 # default
    requests: { cpu: 5m, memory: 16Mi }
    limits: { memory: 64Mi }
  podLabels: {}                              # default
  podAnnotations: {}                         # default
  nodeSelector: {}                           # default
  tolerations: []                            # default
  affinity: {}                               # default

serviceAccount:            # the backend's identity towards the Kubernetes API
  create: true             # default
  name: ""                 # default: <fullname>-backend when created, "default" when not
  annotations: {}          # default

ingress:
  enabled: false           # default
  className: ""            # default; example: haproxy
  hosts: []                # default; example: [dash.example.com]; empty: one rule without a host
  tls: []                  # default; example: [{ secretName: dash-tls, hosts: [dash.example.com] }]
  annotations: {}          # default; on both objects — authentication belongs here
  frontend:
    annotations: {}        # default; on <fullname>-frontend only, e.g. cert-manager.io/cluster-issuer
  backend:
    annotations: {}        # default; on <fullname>-backend only, e.g. a longer API timeout
```

**Security notes.** `ingress.annotations` reach both objects; an authentication annotation in one
object's own set protects only that half. Both pods run as 65532 on a read-only root file system with
every capability dropped; the frontend mounts no ServiceAccount token. With `cert-manager`, put the
issuer annotation under `ingress.frontend.annotations` only: both objects name the same TLS secret,
and one Certificate is made for it (verified with cert-manager v1.21.3 and haproxy-ingress 0.16.2).

</details>

### Backend configuration

| Variable | Default | Meaning |
|---|---|---|
| `DASH_CONFIG` | `/etc/dash/config.yaml` | The YAML file. Missing at the default path: the defaults apply; missing where the variable points: the backend refuses to start |
| `DASH_LISTEN_ADDR` | `:8080` | `host:port` the server binds |
| `DASH_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |

The YAML file holds **no key yet**: every key it sets is refused, and the backend names every invalid
variable and key at once. The chart renders it into `<fullname>-config`.

### Endpoints

| Path | Served by | Answer |
|---|---|---|
| `/api/v1/version` | backend | `{"version","commit","build_time"}` |
| `/api/v1/openapi.json` | backend | The API document |
| `/healthz` | backend, frontend | `200 {"status":"ok"}` while the process serves |
| `/readyz` | backend | `200` once the Kubernetes API server has answered since the start, `503 not_ready` before |

### Problem codes

Every API error is an RFC 9457 `application/problem+json` body with a stable `code` and the request id.

<!-- problem-codes:start -->
| `code` | Status | Meaning |
|---|---|---|
| `validation_failed` | 400 | The request does not match the API document: an unknown query parameter, or a value the document refuses; `errors[]` names each |
| `not_found` | 404 | No such route. The frontend's file server answers the same for an `/api/` path that reached it instead of the backend |
| `method_not_allowed` | 405 | The path exists with other methods; `Allow` names them |
| `internal` | 500 | Something failed inside the backend; the `request_id` finds it in the log |
| `not_ready` | 503 | The backend has not reached the Kubernetes API server since it started (`/readyz`) |
<!-- problem-codes:end -->

## 🛠 Development

```bash
make help                 # every target
make test                 # Go unit tests of both modules and the frontend's
make lint frontend-lint   # static analysis
make e2e                  # the end-to-end suite in a kind cluster of its own
```

More in [docs/developer/](docs/developer/README.md).

## License

[Apache License 2.0](LICENSE). Third-party material: [ATTRIBUTION.md](ATTRIBUTION.md).
