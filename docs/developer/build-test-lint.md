# Build, test, lint

Everything runs through `make` from the repository root (`make help` lists every target). Go targets
cover both modules, `backend/` and `frontend/server/`; npm targets run in `frontend/`. Every tool is
built into `bin/` at its pinned version on first use ([ADR 0006](../adr/0006-one-version-of-every-tool.md)).

## Needs

Go 1.27.3 (an older Go downloads it: `GOTOOLCHAIN=auto`), Node.js 26, Docker. Helm, kind,
kubeconform, helm-unittest and the Go linters come from the Makefile — never from the PATH.

## The targets a change usually needs

| What | Target |
|---|---|
| Format | `make fmt` |
| Static analysis | `make lint cyclo gosec vuln`, `make frontend-lint` |
| Generated code | `make generate` after an API change, `make frontend-generate` after it; the `-check` targets prove it is current |
| Go unit tests | `make test-unit` |
| Integration tests | `make test-integration` (fetches envtest's binaries once) |
| Frontend tests | `make frontend-test` |
| Chart | `make helm-lint helm-template helm-unittest helm-kubeconform`; `helm-template` also runs the backend's validation on each rendered `config.yaml` (needs Go) |
| Images | `make docker-build` (both, then the no-shell proof); the backend's build stage builds the icon packs |
| Icon packs | `make icon-packs` (into `.cache/icons/packs`, for `make run`), `make icon-pins` (the SHA-256 of a logo catalogue's commit) |
| End to end | `make e2e` (a kind cluster of its own, deleted afterwards) |
| Release tooling | `npm ci && make test-release-tooling` |

## Local development

- `make e2e-up` keeps the end-to-end cluster with Dash installed; Dash answers at
  `https://localhost:18443` (self-signed). `make e2e-test` runs the suite against it, `make e2e-down`
  deletes it.
- `make run` runs the backend from source on `:8080` against that cluster's kubeconfig
  (`RUN_KUBECONFIG=` for another); `make frontend-serve` runs the Angular dev server on `:4200`,
  proxying `/api` to it (`frontend/proxy.conf.mjs`).
- Kubeconfig safety: every target that talks to a cluster uses `.cache/kind/kubeconfig`; nothing reads
  or writes `~/.kube/config`.
