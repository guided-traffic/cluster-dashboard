# Repository layout

Read against the tree on 2026-10-26.

```text
dash/
├── backend/                      # Go module github.com/guided-traffic/dash/backend
│   ├── api/                      # the API document (spec-first, docs/adr/0001 D5)
│   │   ├── openapi.yaml          # the root; meta.yaml the version and document paths
│   │   ├── components/           # schemas, responses; problem-codes.yaml (generated)
│   │   ├── oapi-codegen.yaml     # the generator's configuration
│   │   ├── openapi.gen.json      # the bundled document (generated), embedded and served
│   │   └── embed.go
│   ├── cmd/dash/                 # the binary: serve, version
│   ├── internal/
│   │   ├── api/                  # the request pipeline and the handlers
│   │   │   └── apigen/           # oapi-codegen output (generated)
│   │   ├── config/               # DASH_* variables and the strict YAML file → Config
│   │   ├── httpserver/           # health, readiness, request id, recovery, request log, lifecycle
│   │   ├── kube/                 # the REST configuration and the API server probe
│   │   ├── problem/              # RFC 9457 bodies and the code catalogue
│   │   └── requestid/            # the request id in the context
│   ├── test/integration/         # build tag integration: envtest
│   ├── tools/problemdoc/         # writes problem-codes.yaml and the README table
│   ├── tools/specbundle/         # bundles api/ into openapi.gen.json
│   ├── Containerfile             # golang:1.27.3-alpine → distroless static nonroot
│   └── .golangci.yml
├── frontend/                     # Angular 22.2 workspace, project "frontend"
│   ├── src/app/
│   │   ├── api/                  # ng-openapi-gen output (generated)
│   │   ├── core/                 # theme.service, version.service
│   │   ├── design/               # tokens.css (Cogpit's palettes), glyphs.ts (Material Symbols)
│   │   └── shared/theme-control/ # the segmented control of the colour scheme
│   ├── public/                   # favicon.svg, theme-init.js
│   ├── e2e/                      # Playwright: playwright.config.ts, smoke.spec.ts
│   ├── server/                   # Go module …/frontend/server, standard library only: the image's file server
│   ├── proxy.conf.mjs            # ng serve → backend for /api: the developer's stand-in for the Ingress
│   └── Containerfile             # node:26-alpine → golang:1.27.3-alpine → distroless static nonroot
├── deploy/helm/dash/             # the chart: templates/, values.yaml, values.schema.json, ci/, tests/
├── hack/
│   ├── e2e.sh                    # the end-to-end tier: kind, cert-manager, haproxy-ingress, Dash, Playwright
│   ├── kind/                     # the cluster, the controller's values, the issuer, Dash's e2e values
│   ├── envtest/fetch.sh          # the envtest binaries, checksum verified (from Cogpit)
│   ├── no-shell.sh               # proves an image holds no shell
│   ├── coverage.py               # merges Go profiles of both modules and runs
│   └── verify-release-tooling.mjs
├── docs/                         # adr/, developer/, security/, planning/
├── .github/                      # workflows/{release,build,renovate}.yml, release-template.hbs, badges/
├── Makefile, renovate.json, .releaserc.json, package.json (semantic-release)
└── AGENTS.md, README.md, SECURITY.md, ATTRIBUTION.md, LICENSE
```

**Generated and committed**: `backend/api/openapi.gen.json`, `backend/api/components/problem-codes.yaml`,
`backend/internal/api/apigen/`, the problem-code table in `README.md` (between its markers) — written
only by `make generate`, checked by `make generate-check`; `frontend/src/app/api/` — written by
`make frontend-generate`, checked by `make frontend-generate-check`.

**Never committed**: `bin/`, `.cache/`, `coverage/`, `node_modules/`, `dist/`, the e2e results.
