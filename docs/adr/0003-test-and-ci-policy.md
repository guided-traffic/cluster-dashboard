# ADR 0003: Test, Verification and CI Policy

## Status

Accepted 2026-10-26, cowork's ADR 0003 adapted. **Built**: every tier below runs locally; verified
2026-10-26 by running each target (see [docs/developer/testing.md](../developer/testing.md)).
**Not verified:** the workflows on GitHub — no run has happened yet.

## Context

Tests that CI skipped failed unnoticed in the sibling projects; a gate that was not required did not
gate. Dash inherits those rules from its first commit.

## Decision

**D1 — `make` is the entry point**, in CI and locally. Every CI job runs a Makefile target; every
tool is pinned in the Makefile and built into `bin/` under a versioned name ([ADR 0006](0006-one-version-of-every-tool.md)).

**D2 — The tiers.**

| Tier | Target | Needs |
|---|---|---|
| Go unit, both modules | `make test-unit` | nothing |
| Backend integration (tag `integration`) | `make test-integration` | envtest's kube-apiserver and etcd (`make envtest-bin`) |
| Frontend unit | `make frontend-test` | Node.js 26 |
| Chart | `make helm-lint helm-template helm-unittest helm-kubeconform` | the pinned Helm |
| Container | the container scan job: Trivy and `hack/no-shell.sh` per image | Docker |
| End-to-end | `make e2e` | Docker; kind, haproxy-ingress, cert-manager, Playwright (pinned) |
| Release tooling | `make test-release-tooling` | Node.js |

**D3 — No `-short`, no `testing.Short()`, no skip on a missing dependency.** The integration tier fails
without its binaries and says how to get them.

**D4 — A gate that is not required is not a gate.** `semantic-release` lists every verification job in
`needs:`; a new job is added there in the change that adds it.

**D5 — Coverage is measured and shown, not gated by a number**: the merged Go profile and the frontend
lines are a sticky PR comment and the badge.

**D6 — Static analysis is part of the build**: `go vet`, golangci-lint, gosec, govulncheck, gocyclo at
15 (tests excluded), angular-eslint.

**D7 — Supply chain**: ClamAV over the source, Trivy over each image, `npm audit signatures` before
the release tooling runs.

**D8 — A fix comes with the test that failed without it.**

## Consequences

- CI builds the Go tools on every run; the e2e job creates a kind cluster per run.
- No threshold means coverage can drift; the comment shows it on every pull request.

## References

- [Makefile](../../Makefile), [.github/workflows/release.yml](../../.github/workflows/release.yml)
