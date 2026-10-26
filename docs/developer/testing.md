# Testing

The tiers are [ADR 0003](../adr/0003-test-and-ci-policy.md)'s. No `-short`, no skip on a missing
dependency; a fix comes with the test that failed without it.

| Tier | Target | Where | Proves today |
|---|---|---|---|
| Go unit | `make test-unit` | `backend/**/*_test.go`, `frontend/server/server_test.go` | Configuration (every invalid key at once, with its line; every validation rule; the chart's defaults equal to the backend's), the icon reference and badge entry grammars; the badge engine (every match field, their conjunction, a field an item lacks, corners, own entries first, the glyphs at start); the discovery (every annotation key, a `hajimari.io/` key changing nothing, the selection matrix, URL derivation, status, the informers on a fake clientset in each namespace layout, a forbidden list logged and never ready) and the dashboard model (groups, order, what the configuration hides); the icons (the reference grammar, the content check, packs, every fallback of the resolution, auto-match, ConfigMap icons in the kubelet's layout, the fetcher's address matrix against TLS servers on loopback, redirects, bounds, the remote table) and the pack builder (fixtures for both kinds of catalogue, every verification, `required.txt`); the HTTP server (health, readiness, request id, recovery, the log line), the API against its document, problem bodies, the probe, the generators; the file server's rules (router paths, cache headers, CSP, the `/api/` problem, no listing, no path outside the bundle, ranges, gzip) |
| Backend integration | `make test-integration` | `backend/test/integration/` | The backend's client reaches a real kube-apiserver (envtest `ENVTEST_K8S_VERSION`); the discovery runs as the chart's ServiceAccount under exactly the RBAC `helm template` renders for every namespace, for named namespaces and for a label selector, sees what each selects, follows changes within a bound, and is refused Secrets, ConfigMaps and what its mode does not grant; with a badge rule it reads the default IngressClass and its controller under the extra ClusterRole |
| Frontend unit | `make frontend-test` | `frontend/src/**/*.spec.ts` | The theme service (the installation default included), the theme control, the version service, the dashboard service (polling, visibility, a failed refresh), the clock, every component of the page — icon (mask, image, dark variant), badge (corner, glyph or text, tone, chip), bracket, tile (link, status words, badges), summary, bookmarks, horizon, the page in each of its states — the shell, `theme-init.js`, the tokens (every text, status and badge-tone pair of both schemes) |
| Chart | `make helm-unittest` and friends | `deploy/helm/dash/tests/`, `ci/*-values.yaml` | Both Ingress objects and their annotations, the opt-out, the hardening, the ServiceAccount, the configuration checksum, the rendered configuration file, the RBAC of each namespace-selector mode, the icon ConfigMaps and their mounts; the schema refusing invalid values (`HELM_REFUSED_VALUES`); the `config.yaml` of every `ci/*-values.yaml` accepted by the backend's validation (`backend/tools/configcheck`) |
| Container | CI's container scan, `make docker-build` | `hack/no-shell.sh` | Each image builds from its directory, passes Trivy, holds no shell |
| End to end | `make e2e` | `hack/e2e.sh`, `frontend/e2e/` | In kind behind haproxy-ingress with cert-manager: the shell and its headers through the frontend object, the version through the backend object, a deep link, both colour schemes, a chosen scheme surviving a reload, the served certificate being the one cert-manager issued, `/api/v1/dashboard` through the backend object with `defaultEnable` on and Dash's own two Ingress objects kept off by the chart's opt-out, an auto-matched logo of the image's packs and a key of the chart's icon ConfigMap fetched with their headers, a rule's badge and an own text badge, the page with its tile, loaded logo, badge, summary and bookmark, one column at phone width without horizontal scroll — Chromium and WebKit |
| Release tooling | `make test-release-tooling` | `hack/verify-release-tooling.mjs` | semantic-release still renders notes |

## Fixtures and environment

- Go tests run with `KUBECONFIG=/dev/null` and without `KUBERNETES_SERVICE_*` (`TEST_ENV`), so code
  falling back to client-go's defaults finds no cluster.
- envtest: `hack/envtest/fetch.sh` downloads `envtest-v<version>-<platform>.tar.gz` and refuses it
  unless its SHA-512 is the Makefile's pin (darwin-arm64, linux-amd64). envtest runs no controllers:
  the tests write EndpointSlices themselves.
- The chart's RBAC per mode: `make integration-rbac` renders it into `.cache/rbac/` with the pinned
  Helm (release `dash`, namespace `dash`); the integration tier fails without it.
- The e2e cluster: `hack/kind/cluster.yaml` maps the node's 443 to `127.0.0.1:18443`;
  `hack/kind/dash-values.yaml` installs the loaded images with `pullPolicy: Never` and TLS on
  `localhost`; `hack/e2e.sh` passes the certificate's SHA-256 fingerprint to the suite.

## Coverage

Measured, shown on every pull request, not gated: `make test-unit-coverage test-integration-coverage
coverage-report` merges both modules' profiles (`hack/coverage.py`); `make frontend-test-coverage`
writes the frontend's. The generated client and `main.ts` are excluded from the frontend's.
