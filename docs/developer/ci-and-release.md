# CI and release

## Test and Release (`.github/workflows/release.yml`)

On every pull request and push to `main`, on the organisation's self-hosted runners. Every job runs
a make target; `semantic-release` needs every verification job ([ADR 0003](../adr/0003-test-and-ci-policy.md) D4).

| Job | Runs |
|---|---|
| Code Linting | `make lint generate-check` |
| GoSec Security Scan | `make gosec` |
| Vulnerability Check | `make vuln` |
| Cyclomatic Complexity | `make cyclo` (report in the summary) |
| Malware Scan (Source Code) | ClamAV over the checkout |
| Unit Tests | `make test-unit-coverage` |
| Integration Tests (envtest) | `make test-integration-coverage` |
| Frontend | `make frontend-install frontend-generate-check frontend-lint frontend-test-coverage frontend-build` |
| Helm Chart | `make helm-lint helm-template helm-unittest helm-kubeconform` |
| Combined Coverage Report | `make coverage-report`; the sticky PR comment; the badge on `main` |
| Container Malware Scan (backend, frontend) | build, Trivy (CRITICAL/HIGH fails), `hack/no-shell.sh`, the image saved for e2e |
| End-to-End Tests | `make e2e E2E_BUILD=false` with the scanned images |
| Release Tooling | `npm ci`, `npm audit signatures`, `make test-release-tooling` |
| Semantic Release | on `main` after all of the above, with the GitHub App's token |

## Release Docker & Helm (`.github/workflows/build.yml`)

On a published release: both images to Docker Hub as `guidedtraffic/dash-backend` and
`guidedtraffic/dash-frontend` (semver tags, `latest`, SBOM, provenance, Docker Scout), then the chart
packaged at the release's version and merged into the index on `gh-pages`, served at
`https://guided-traffic.github.io/dash/`.

## Renovate (`.github/workflows/renovate.yml`, `renovate.json`)

Daily at 02:00 Europe/Berlin with the GitHub App's token. The rules are
[ADR 0006](../adr/0006-one-version-of-every-tool.md) D4.

## Secrets and settings

The organisation secrets `APP_CLIENT_ID`, `APP_PRIVATE_KEY` and `DOCKERHUB_PAT` are visible to the
repository, and `gh-pages` with Pages exists (verified 2026-10-26). **Open:** no ruleset protects
`main` yet — the required checks can be named after the first run; the Docker Hub repositories
`guidedtraffic/dash-backend` and `-frontend` did not exist on 2026-10-26, and whether the first
release's push creates them is not verified.

## Commits

Conventional commits, no apostrophe in a message (semantic-release reads them). `feat` releases a
minor, `fix` a patch; dependency updates are `fix` so a release ships them, GitHub Actions `chore`.
