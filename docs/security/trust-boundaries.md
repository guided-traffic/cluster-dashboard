# Trust boundaries

```text
            ┌──────────────────────────── cluster ─────────────────────────────┐
browser ──► │ Ingress controller ─┬─► frontend (file server)                    │
 (anyone    │  (one hop, TLS)     └─► backend ──► Kubernetes API               │
  who can   │                           ▲   (version; list and watch:          │
  reach it) │                           │    Ingresses, EndpointSlices)        │
            │   Ingress authors ────────┘ (annotations, read by the discovery) │
            └──────────────────────────────────────────────────────────────────┘
```

| Boundary | Crosses | What the receiving side trusts |
|---|---|---|
| Browser → controller | HTTPS, if the installation configures TLS | Nothing: no login ([exposure.md](exposure.md)) |
| Controller → frontend | `/` | Nothing from the request; it serves a fixed bundle from memory |
| Controller → backend | `/api/` | Requests are validated against the OpenAPI document; unknown parameters are refused |
| Backend → Kubernetes API | the pod's ServiceAccount token | The API server's certificate, from the token's CA bundle; the read is RBAC's ([privilege-footprint.md](privilege-footprint.md)) |
| Ingress authors → backend | `dash.gtrfc.com/*` annotations of Ingresses in watched namespaces | Nothing: each key is parsed, an unusable value is ignored with a log line naming the key, never the value ([validation.md](validation.md)) |

- Neither image has a shell, a package manager or a libc; both run as 65532 on a read-only root file
  system ([privilege-footprint.md](privilege-footprint.md)).
- The frontend never calls the backend; the browser does, through the controller, on one origin.
- Every UI response carries a Content-Security-Policy with every source `'self'` — `'unsafe-inline'`
  for styles only — and `frame-ancestors 'none'`.

## What this does not cover

- The controller's own configuration (body limits, timeouts, TLS versions) is the installation's.
- Icons fetched from URLs are phase 2; their boundary is added with them.
