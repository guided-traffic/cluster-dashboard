# Dash — the one entry point, locally and in CI (docs/adr/0003 D1): every CI job runs
# a target of this file, and a developer runs the same target and gets the same result.
#
# One version of every tool (docs/adr/0006): each tool is pinned once below with a `# renovate:`
# comment, built or downloaded into bin/ under a file name that carries its version, and built
# again when its pin or the Go toolchain changes (its stamp in bin/.stamps names both). Nothing
# here runs a tool from the PATH that has a pin: not helm, not kind, not kubeconform.
#
# Kubeconfig safety: every target that talks to a cluster uses KIND_KUBECONFIG, a file of its own
# under .cache/kind/, and the kind cluster KIND_CLUSTER; nothing here reads or writes
# ~/.kube/config or another context. The Go tests run without KUBERNETES_SERVICE_HOST and
# KUBERNETES_SERVICE_PORT and with KUBECONFIG=/dev/null (TEST_ENV), so that code falling back to
# client-go's defaults finds no cluster — CI's self-hosted runners are pods with a ServiceAccount
# of their own.

# Setting SHELL to bash allows pipefail and the [[ ]] the recipes use.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# go.mod names the toolchain (go 1.27.3); an older local Go downloads it.
export GOTOOLCHAIN := auto

# Image names. make docker-build tags both images with these; make e2e loads them into kind.
BACKEND_IMG ?= guidedtraffic/dash-backend:dev
FRONTEND_IMG ?= guidedtraffic/dash-frontend:dev

# Build metadata baked into the backend binary (backend/cmd/dash/main.go); both
# Containerfiles take the same three values as build arguments.
VERSION ?= dev
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_TIME ?= $(shell date +%s)
LDFLAGS = -w -s -X main.version=$(VERSION) -X main.commit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)

# Directories. The Go targets cover both Go modules: the backend and the frontend's file server.
BACKEND_DIR := backend
FRONTEND_DIR := frontend
SERVER_DIR := frontend/server
COVERAGE_DIR := $(CURDIR)/coverage
BIN := $(CURDIR)/bin
CACHE := $(CURDIR)/.cache
HELM_CHART := deploy/helm/dash

# The Go tests never reach a cluster by accident (the kubeconfig safety above).
TEST_ENV := env -u KUBERNETES_SERVICE_HOST -u KUBERNETES_SERVICE_PORT KUBECONFIG=/dev/null

# Generated Go code is excluded from the complexity scan by name; the drift check covers it.
GENERATED_GO := _test\.go|\.gen\.go
# Cyclomatic complexity threshold (docs/adr/0003 D6).
CYCLO_THRESHOLD ?= 15
# gosec bounds: the self-hosted runners share one machine across jobs.
GOSEC_CONCURRENCY ?= 4
GOSEC_MEMLIMIT ?= 1GiB

## Tool versions — one pin each, read by every target and by CI through make.

# renovate: datasource=go depName=github.com/golangci/golangci-lint/v2/cmd/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.15.0
# renovate: datasource=go depName=github.com/fzipp/gocyclo/cmd/gocyclo
GOCYCLO_VERSION ?= v0.6.0
# renovate: datasource=go depName=github.com/securego/gosec/v2/cmd/gosec
GOSEC_VERSION ?= v2.30.0
# renovate: datasource=go depName=golang.org/x/vuln/cmd/govulncheck
GOVULNCHECK_VERSION ?= v1.9.0
# renovate: datasource=go depName=github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen
OAPI_CODEGEN_VERSION ?= v2.8.1
# renovate: datasource=go depName=github.com/yannh/kubeconform/cmd/kubeconform
KUBECONFORM_VERSION ?= v0.8.0
# Helm 4, built from its module with its version stamped in as its release build does; never the
# helm on the PATH (docs/adr/0006 D3).
# renovate: datasource=go depName=helm.sh/helm/v4
HELM_VERSION ?= v4.3.1
# helm-unittest, built from its own module (it carries a replace directive, so go install cannot
# build it) and run as its own binary: it renders with the Helm 4 engine it links.
# renovate: datasource=go depName=github.com/helm-unittest/helm-unittest
HELM_UNITTEST_VERSION ?= v1.2.1
# kind and the node image of its release; both move together (renovate.json, group kind).
# renovate: datasource=go depName=sigs.k8s.io/kind
KIND_VERSION ?= v0.33.0
# renovate: datasource=docker depName=kindest/node
KIND_NODE_IMAGE ?= kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5
# The end-to-end cluster's Ingress controller — the one the owner's clusters run — and cert-manager.
# renovate: datasource=helm depName=haproxy-ingress registryUrl=https://haproxy-ingress.github.io/charts
HAPROXY_INGRESS_CHART_VERSION ?= 0.16.2
HAPROXY_INGRESS_CHART_REPO := https://haproxy-ingress.github.io/charts
# renovate: datasource=helm depName=cert-manager registryUrl=https://charts.jetstack.io
CERT_MANAGER_CHART_VERSION ?= v1.21.3
CERT_MANAGER_CHART_REPO := https://charts.jetstack.io
# The Kubernetes version the chart's manifests are validated against (helm-kubeconform): the
# version kind runs and envtest starts.
KUBE_VERSION := 1.37.0

## Tool binaries: each path carries its version, so a bump is a missing file that installs itself.
GOLANGCI_LINT := $(BIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
GOCYCLO := $(BIN)/gocyclo-$(GOCYCLO_VERSION)
GOSEC := $(BIN)/gosec-$(GOSEC_VERSION)
GOVULNCHECK := $(BIN)/govulncheck-$(GOVULNCHECK_VERSION)
OAPI_CODEGEN := $(BIN)/oapi-codegen-$(OAPI_CODEGEN_VERSION)
KUBECONFORM := $(BIN)/kubeconform-$(KUBECONFORM_VERSION)
HELM_BIN := $(BIN)/helm-$(HELM_VERSION)
HELM_UNITTEST := $(BIN)/helm-unittest-$(HELM_UNITTEST_VERSION)
KIND := $(BIN)/kind-$(KIND_VERSION)

# A tool is built again when the Go toolchain changes: its rule depends on a stamp that names the
# tool's version and the toolchain go.mod selects; a new pair is a missing stamp.
GO_TOOLCHAIN := $(shell cd $(BACKEND_DIR) && go env GOVERSION 2>/dev/null)
STAMPS := $(BIN)/.stamps
tool_stamp = $(STAMPS)/$(1)@$(2)-$(GO_TOOLCHAIN)

# Helm with a home of its own: no repository, plugin or setting of the machine's Helm is read.
HELM := HELM_CACHE_HOME=$(CACHE)/helm/cache HELM_CONFIG_HOME=$(CACHE)/helm/config HELM_DATA_HOME=$(CACHE)/helm/data $(HELM_BIN)

## The integration tier: envtest (docs/adr/0003 D2). A real kube-apiserver and etcd, no Docker and
# no kubelet, from envtest's release archive of Kubernetes ENVTEST_K8S_VERSION in
# controller-tools' release store — the one setup-envtest downloads — pinned by version and by the
# SHA-512 its index publishes per platform, kept in .cache/envtest/<version>
# (hack/envtest/fetch.sh, from Cogpit). Both SHA-512 were taken on 2026-10-26 from
# https://raw.githubusercontent.com/kubernetes-sigs/controller-tools/HEAD/envtest-releases.yaml;
# Renovate proposes a new version and its pull request fails on the checksum until the two pins
# below are taken from that index by hand.
# renovate: envtest
ENVTEST_K8S_VERSION ?= 1.37.0
ENVTEST_SHA512_Darwin_arm64 := fb38cfacdd71b5e97a4d4cceac861af5f55069cf783f0e49cf181bfc32eb3e557c2091a534dc5d38f1b92c5ba142bc1979f215a5385b3630ea6a661be6fa161b
ENVTEST_PLATFORM_Darwin_arm64 := darwin-arm64
ENVTEST_SHA512_Linux_x86_64 := 1d1c453633b72c161a5d5a886cde7ac850be1a2ac796a9e1d4ffacacc64868295bdd2d57aa66cd0c158f5ce510f5dfe3fbc61ac21bd3dcfb875bd70658aa663a
ENVTEST_PLATFORM_Linux_x86_64 := linux-amd64
ENVTEST_HOST = $(shell uname -s)_$(shell uname -m)
ENVTEST_DIR := $(CACHE)/envtest/$(ENVTEST_K8S_VERSION)

## The end-to-end tier: kind (docs/adr/0003 D2).
KIND_CLUSTER ?= dash-e2e
KIND_KUBECONFIG ?= $(CACHE)/kind/kubeconfig
# The HTTPS port of the cluster's Ingress controller on 127.0.0.1.
E2E_PORT ?= 18443
# false: make e2e takes BACKEND_IMG and FRONTEND_IMG as they are, as CI does with the images its
# container scan built; true builds them first.
E2E_BUILD ?= true
E2E_ENV = KIND="$(KIND)" HELM="$(HELM)" KIND_CLUSTER=$(KIND_CLUSTER) KIND_KUBECONFIG=$(KIND_KUBECONFIG) \
	KIND_NODE_IMAGE=$(KIND_NODE_IMAGE) E2E_PORT=$(E2E_PORT) BACKEND_IMG=$(BACKEND_IMG) FRONTEND_IMG=$(FRONTEND_IMG) \
	HAPROXY_INGRESS_CHART_VERSION=$(HAPROXY_INGRESS_CHART_VERSION) HAPROXY_INGRESS_CHART_REPO=$(HAPROXY_INGRESS_CHART_REPO) \
	CERT_MANAGER_CHART_VERSION=$(CERT_MANAGER_CHART_VERSION) CERT_MANAGER_CHART_REPO=$(CERT_MANAGER_CHART_REPO) \
	HELM_CHART=$(HELM_CHART)

.PHONY: all
all: build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-26s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: tools
tools: $(GOLANGCI_LINT) $(GOCYCLO) $(GOSEC) $(GOVULNCHECK) $(OAPI_CODEGEN) $(KUBECONFORM) $(HELM_BIN) $(HELM_UNITTEST) $(KIND) ## Build every pinned tool into bin/.

##@ Go (both modules: backend/ and frontend/server/)

.PHONY: generate
generate: $(OAPI_CODEGEN) ## Regenerate the problem-code enum and README table, the bundled API document and the Go server interface.
	cd $(BACKEND_DIR) && go run ./tools/problemdoc
	cd $(BACKEND_DIR) && go run ./tools/specbundle
	cd $(BACKEND_DIR) && $(OAPI_CODEGEN) -config api/oapi-codegen.yaml api/openapi.gen.json

.PHONY: generate-check
generate-check: generate ## Fail when a generated file differs from what make generate writes.
	@git diff --exit-code -- $(BACKEND_DIR) README.md || { echo "generated files are out of date: run make generate and commit the result"; exit 1; }
	@untracked=$$(git ls-files --others --exclude-standard -- $(BACKEND_DIR)); if [ -n "$$untracked" ]; then echo "make generate wrote untracked files:"; echo "$$untracked"; exit 1; fi

.PHONY: fmt
fmt: $(GOLANGCI_LINT) ## Format both Go modules (gofmt, goimports).
	cd $(BACKEND_DIR) && $(GOLANGCI_LINT) fmt ./...
	cd $(SERVER_DIR) && $(GOLANGCI_LINT) fmt ./...

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Static analysis of both Go modules: go vet, golangci-lint (with the integration tag for backend/test).
	cd $(BACKEND_DIR) && go vet ./... && go vet -tags=integration ./test/...
	cd $(BACKEND_DIR) && $(GOLANGCI_LINT) run --timeout=5m ./...
	cd $(BACKEND_DIR) && $(GOLANGCI_LINT) run --timeout=5m --build-tags=integration ./test/...
	cd $(SERVER_DIR) && go vet ./...
	cd $(SERVER_DIR) && $(GOLANGCI_LINT) run --timeout=5m ./...

.PHONY: cyclo
cyclo: $(GOCYCLO) ## Fail on any function of either module above the cyclomatic complexity threshold.
	@echo "Running cyclomatic complexity analysis (threshold: $(CYCLO_THRESHOLD))..."
	@out=$$(cd $(BACKEND_DIR) && $(GOCYCLO) -over $(CYCLO_THRESHOLD) -ignore "$(GENERATED_GO)" api cmd internal test tools; cd $(CURDIR)/$(SERVER_DIR) && $(GOCYCLO) -over $(CYCLO_THRESHOLD) -ignore "$(GENERATED_GO)" .) || true; \
	if [ -n "$$out" ]; then echo "$$out"; echo "Functions above complexity threshold $(CYCLO_THRESHOLD) found!"; exit 1; fi; \
	echo "All functions are below complexity threshold $(CYCLO_THRESHOLD)"

.PHONY: cyclo-report
cyclo-report: $(GOCYCLO) ## Show the 20 most complex functions of each module, tests included.
	@cd $(BACKEND_DIR) && $(GOCYCLO) -top 20 -ignore "\.gen\.go" api cmd internal test tools
	@cd $(SERVER_DIR) && $(GOCYCLO) -top 20 .

.PHONY: gosec
gosec: $(GOSEC) ## Run the gosec security scan on both modules.
	cd $(BACKEND_DIR) && GOFLAGS="-buildvcs=false -p=$(GOSEC_CONCURRENCY)" GOMEMLIMIT=$(GOSEC_MEMLIMIT) $(GOSEC) -concurrency=$(GOSEC_CONCURRENCY) -exclude-generated ./...
	cd $(SERVER_DIR) && GOFLAGS="-buildvcs=false -p=$(GOSEC_CONCURRENCY)" GOMEMLIMIT=$(GOSEC_MEMLIMIT) $(GOSEC) -concurrency=$(GOSEC_CONCURRENCY) ./...

.PHONY: vuln
vuln: $(GOVULNCHECK) ## Check both modules' dependencies and standard library for known vulnerabilities.
	cd $(BACKEND_DIR) && $(GOVULNCHECK) ./...
	cd $(SERVER_DIR) && $(GOVULNCHECK) ./...

# No -short and no testing.Short() gates: a test that CI never runs is not a test (docs/adr/0003 D3).
.PHONY: test-unit
test-unit: ## Run the unit tests of both modules, race detector on (nothing running needed).
	cd $(BACKEND_DIR) && $(TEST_ENV) go test -race -count=1 ./...
	cd $(SERVER_DIR) && $(TEST_ENV) go test -race -count=1 ./...

.PHONY: test-unit-coverage
test-unit-coverage: ## Run the unit tests of both modules with one coverage profile in coverage/unit.out.
	@mkdir -p $(COVERAGE_DIR)
	cd $(BACKEND_DIR) && $(TEST_ENV) go test -race -count=1 -covermode=atomic -coverpkg=./... -coverprofile=$(COVERAGE_DIR)/unit-backend.out ./...
	cd $(SERVER_DIR) && $(TEST_ENV) go test -race -count=1 -covermode=atomic -coverprofile=$(COVERAGE_DIR)/unit-server.out ./...
	{ echo "mode: atomic"; tail -n +2 $(COVERAGE_DIR)/unit-backend.out; tail -n +2 $(COVERAGE_DIR)/unit-server.out; } > $(COVERAGE_DIR)/unit.out
	@python3 hack/coverage.py total $(COVERAGE_DIR)/unit.out

.PHONY: envtest-bin
envtest-bin: $(ENVTEST_DIR)/.verified ## Download the pinned envtest binaries (etcd, kube-apiserver) into .cache/envtest, checksum verified.

$(ENVTEST_DIR)/.verified:
	hack/envtest/fetch.sh '$(ENVTEST_K8S_VERSION)' '$(ENVTEST_PLATFORM_$(ENVTEST_HOST))' '$(ENVTEST_SHA512_$(ENVTEST_HOST))' '$(ENVTEST_DIR)'

.PHONY: envtest-key
envtest-key: ## Print the CI cache key of the envtest binaries: version, platform and pinned SHA-512.
	@echo 'envtest-$(ENVTEST_K8S_VERSION)-$(ENVTEST_PLATFORM_$(ENVTEST_HOST))-$(ENVTEST_SHA512_$(ENVTEST_HOST))'

INTEGRATION_ENV = $(TEST_ENV) DASH_TEST_ENVTEST_ASSETS=$(ENVTEST_DIR) DASH_TEST_ENVTEST_VERSION=$(ENVTEST_K8S_VERSION)

.PHONY: test-integration
test-integration: envtest-bin ## Run the backend integration tests (tag integration) against envtest's kube-apiserver and etcd.
	cd $(BACKEND_DIR) && $(INTEGRATION_ENV) go test -race -tags=integration -count=1 -timeout=10m ./test/integration/...

.PHONY: test-integration-coverage
test-integration-coverage: envtest-bin ## Run the backend integration tests with a coverage profile in coverage/integration.out.
	@mkdir -p $(COVERAGE_DIR)
	cd $(BACKEND_DIR) && $(INTEGRATION_ENV) go test -race -tags=integration -count=1 -timeout=10m -covermode=atomic -coverpkg=./... -coverprofile=$(COVERAGE_DIR)/integration.out ./test/integration/...
	@python3 hack/coverage.py total $(COVERAGE_DIR)/integration.out

.PHONY: coverage-report
coverage-report: ## Merge coverage/unit.out and coverage/integration.out into coverage/summary.md and coverage/badge.json.
	python3 hack/coverage.py report $(COVERAGE_DIR) $(wildcard $(COVERAGE_DIR)/unit.out $(COVERAGE_DIR)/integration.out)

.PHONY: build-backend
build-backend: ## Build bin/dash.
	cd $(BACKEND_DIR) && CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BIN)/dash ./cmd/dash

.PHONY: build-frontend-server
build-frontend-server: ## Build bin/frontend-server, the frontend image's file server.
	cd $(SERVER_DIR) && CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o $(BIN)/frontend-server .

# The backend outside a cluster reads the kubeconfig client-go's default loading rules find; this
# target hands it the kind cluster's (make e2e-up), never ~/.kube/config. RUN_KUBECONFIG= names
# another file.
RUN_KUBECONFIG ?= $(KIND_KUBECONFIG)

.PHONY: run
run: ## Run the backend from source on :8080 against the kind cluster of make e2e-up (RUN_KUBECONFIG= for another).
	cd $(BACKEND_DIR) && env -u KUBERNETES_SERVICE_HOST -u KUBERNETES_SERVICE_PORT KUBECONFIG=$(RUN_KUBECONFIG) DASH_LOG_LEVEL=$${DASH_LOG_LEVEL:-debug} go run -ldflags="$(LDFLAGS)" ./cmd/dash serve

##@ Frontend

# npm ci runs only when the lockfile changed; npm writes this marker on success.
$(FRONTEND_DIR)/node_modules/.package-lock.json: $(FRONTEND_DIR)/package-lock.json
	cd $(FRONTEND_DIR) && npm ci --no-audit --no-fund

.PHONY: frontend-install
frontend-install: $(FRONTEND_DIR)/node_modules/.package-lock.json ## Install the frontend dependencies (npm ci, when the lockfile changed).

.PHONY: frontend-generate
frontend-generate: frontend-install ## Regenerate the Angular API client in frontend/src/app/api from backend/api/openapi.gen.json.
	cd $(FRONTEND_DIR) && npx ng-openapi-gen -c ng-openapi-gen.json

.PHONY: frontend-generate-check
frontend-generate-check: frontend-generate ## Fail when the generated Angular API client differs from what make frontend-generate writes.
	@git diff --exit-code -- $(FRONTEND_DIR)/src/app/api || { echo "frontend/src/app/api is stale: run make frontend-generate and commit the result"; exit 1; }
	@test -z "$$(git ls-files --others --exclude-standard -- $(FRONTEND_DIR)/src/app/api)" || { echo "untracked files in frontend/src/app/api: run make frontend-generate and commit the result"; exit 1; }

.PHONY: frontend-lint
frontend-lint: frontend-install ## Run ESLint (angular-eslint) on the frontend and its end-to-end suite.
	cd $(FRONTEND_DIR) && npx ng lint

.PHONY: frontend-test
frontend-test: frontend-install ## Run the frontend unit tests once (vitest, jsdom).
	cd $(FRONTEND_DIR) && CI=true npx ng test --watch=false

.PHONY: frontend-test-coverage
frontend-test-coverage: frontend-install ## Run the frontend unit tests with coverage in frontend/coverage/frontend/.
	cd $(FRONTEND_DIR) && CI=true npx ng test --watch=false --coverage --coverage-reporters=text-summary --coverage-reporters=lcovonly --coverage-reporters=json-summary

.PHONY: frontend-build
frontend-build: frontend-install ## Build the frontend for production into frontend/dist/frontend/browser.
	cd $(FRONTEND_DIR) && npx ng build --configuration production

.PHONY: frontend-serve
frontend-serve: frontend-install ## Run the Angular dev server on :4200, proxying /api to the backend on :8080 (make run).
	cd $(FRONTEND_DIR) && npx ng serve

##@ Test

.PHONY: test
test: test-unit frontend-test ## Run the unit tests of both Go modules and the frontend.

.PHONY: test-release-tooling
test-release-tooling: ## Verify the semantic-release dependency set renders release notes (npm ci at the root first).
	node hack/verify-release-tooling.mjs

.PHONY: e2e
e2e: $(if $(filter true,$(E2E_BUILD)),docker-build) frontend-install e2e-browsers $(KIND) $(HELM_BIN) ## Run the end-to-end suite in a kind cluster of its own (images built unless E2E_BUILD=false), then delete the cluster.
	$(E2E_ENV) hack/e2e.sh run $(E2E_ARGS)

.PHONY: e2e-up
e2e-up: $(KIND) $(HELM_BIN) ## Create the end-to-end cluster with BACKEND_IMG and FRONTEND_IMG installed, and keep it.
	$(E2E_ENV) hack/e2e.sh up

.PHONY: e2e-test
e2e-test: frontend-install ## Run the end-to-end suite against the cluster of make e2e-up (E2E_ARGS= reaches playwright test).
	$(E2E_ENV) hack/e2e.sh suite $(E2E_ARGS)

.PHONY: e2e-down
e2e-down: $(KIND) ## Delete the end-to-end cluster and its kubeconfig file.
	$(E2E_ENV) hack/e2e.sh down

.PHONY: e2e-browsers
e2e-browsers: frontend-install ## Install the browsers of the end-to-end suite, Chromium and WebKit; PLAYWRIGHT_INSTALL_FLAGS=--with-deps adds their system packages.
	cd $(FRONTEND_DIR) && npx playwright install $(PLAYWRIGHT_INSTALL_FLAGS) chromium webkit

##@ Build

.PHONY: build
build: build-backend build-frontend-server frontend-build ## Build bin/dash, bin/frontend-server and the frontend bundle.

.PHONY: docker-build
docker-build: docker-build-backend docker-build-frontend docker-no-shell ## Build both images and prove neither holds a shell.

.PHONY: docker-build-backend
docker-build-backend: ## Build the backend image from backend/Containerfile, backend/ as its context.
	docker build -f $(BACKEND_DIR)/Containerfile --build-arg BUILD_NUMBER=$(VERSION) --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg BUILD_TIME=$(BUILD_TIME) -t $(BACKEND_IMG) $(BACKEND_DIR)

.PHONY: docker-build-frontend
docker-build-frontend: ## Build the frontend image from frontend/Containerfile, frontend/ as its context.
	docker build -f $(FRONTEND_DIR)/Containerfile --build-arg BUILD_NUMBER=$(VERSION) --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg BUILD_TIME=$(BUILD_TIME) -t $(FRONTEND_IMG) $(FRONTEND_DIR)

.PHONY: docker-no-shell
docker-no-shell: ## Fail unless BACKEND_IMG and FRONTEND_IMG are distroless: no shell to start, none in their files.
	hack/no-shell.sh $(BACKEND_IMG) $(FRONTEND_IMG)

##@ Helm (the pinned Helm 4 in bin/, never the PATH's)

.PHONY: helm
helm: $(HELM_BIN) ## Build the pinned Helm; HELM_ARGS= runs it with its own home, e.g. make helm HELM_ARGS=version.
	$(HELM) $(HELM_ARGS)

.PHONY: helm-lint
helm-lint: $(HELM_BIN) ## Lint the chart strictly, alone and with every values file under ci/.
	$(HELM) lint $(HELM_CHART) --strict
	@for f in $(HELM_CHART)/ci/*-values.yaml; do echo "helm lint with $$f"; $(HELM) lint $(HELM_CHART) --strict -f $$f; done

# Values the schema must refuse, one --set each, quoted for the shell; a comma sets several keys of
# one case, and \, is a comma inside a value.
HELM_REFUSED_VALUES := 'ingress.enabled=yes' 'backend.replicas=-1' 'frontend.image.pullPolicy=Sometimes' \
	'unknownKey=1' 'discovery.namespaceSelector.matchNames={media}' 'discovery.instanceName=a\,b' \
	'ui.defaultTheme=sepia' 'ui.refreshSeconds=4' 'ui.refreshSeconds=3601' 'ui.showSearch=true' \
	'icons.defaultAppIcon=https://example.com/a.png' 'icons.autoMatch.catalogs={mdi}' \
	'icons.remote.timeout=soon' 'icons.remote.maxBytes=10' 'badges.definitions.x.label=neither' \
	'badges.definitions.x.text=toolongtext,badges.definitions.x.label=l' \
	'badges.definitions.x.text=v,badges.definitions.x.label=l,badges.definitions.x.tone=violet' \
	'badges.rules[0].badge=x,badges.rules[0].corner=middle' \
	'apps[0].group=g,apps[0].apps[0].name=n,apps[0].apps[0].url=javascript:alert(1)'

.PHONY: helm-template
helm-template: $(HELM_BIN) ## Render the chart with every values file under ci/, hand each rendered config.yaml to the backend's validation, and check that invalid values are refused.
	@for f in $(HELM_CHART)/ci/*-values.yaml; do echo "helm template with $$f"; \
	  $(HELM) template dash $(HELM_CHART) -f $$f | (cd $(BACKEND_DIR) && go run ./tools/configcheck); done
	@echo "helm template with values the schema refuses: each must fail the render"
	@for set in $(HELM_REFUSED_VALUES); do \
	  if $(HELM) template dash $(HELM_CHART) --set "$$set" >/dev/null 2>&1; then echo "--set $$set must fail against values.schema.json"; exit 1; fi; \
	done

.PHONY: helm-unittest
helm-unittest: $(HELM_UNITTEST) ## Run the chart's helm-unittest suite (deploy/helm/dash/tests).
	$(HELM_UNITTEST) --color=false $(HELM_CHART)

.PHONY: helm-kubeconform
helm-kubeconform: $(HELM_BIN) $(KUBECONFORM) ## Validate the manifests rendered with every values file against the schemas of Kubernetes KUBE_VERSION (needs the network).
	@for f in $(HELM_CHART)/ci/*-values.yaml; do echo "kubeconform with $$f"; \
	  $(HELM) template dash $(HELM_CHART) -f $$f | $(KUBECONFORM) -strict -summary -kubernetes-version $(KUBE_VERSION) -; done

# The release workflow packages the chart with the release's version (build.yml).
CHART_VERSION ?= 0.0.0-dev
HELM_REPO_DIR ?= $(CURDIR)/dist/helm
HELM_REPO_URL ?= https://guided-traffic.github.io/dash/

.PHONY: helm-package
helm-package: $(HELM_BIN) ## Package the chart as CHART_VERSION (version and appVersion) into HELM_REPO_DIR.
	@mkdir -p $(HELM_REPO_DIR)
	$(HELM) package $(HELM_CHART) --version $(CHART_VERSION) --app-version $(CHART_VERSION) --destination $(HELM_REPO_DIR)

.PHONY: helm-index
helm-index: $(HELM_BIN) ## Write HELM_REPO_DIR/index.yaml for HELM_REPO_URL, merged into HELM_INDEX_MERGE when it names an existing index.
	$(HELM) repo index $(HELM_REPO_DIR) --url $(HELM_REPO_URL) $(if $(HELM_INDEX_MERGE),--merge $(HELM_INDEX_MERGE))

##@ Dependencies

$(BIN):
	mkdir -p $(BIN)

# A tool's stamp, made when its version or the toolchain changed: the stamps of its other versions
# go, so the tool's rule, which depends on the new stamp, builds it again.
$(STAMPS)/%:
	@mkdir -p $(STAMPS)
	@rm -f $(STAMPS)/$(firstword $(subst @, ,$*))@*
	@touch $@

$(GOLANGCI_LINT): $(call tool_stamp,golangci-lint,$(GOLANGCI_LINT_VERSION)) | $(BIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

$(GOCYCLO): $(call tool_stamp,gocyclo,$(GOCYCLO_VERSION)) | $(BIN)
	$(call go-install-tool,$(GOCYCLO),github.com/fzipp/gocyclo/cmd/gocyclo,$(GOCYCLO_VERSION))

$(GOSEC): $(call tool_stamp,gosec,$(GOSEC_VERSION)) | $(BIN)
	$(call go-install-tool,$(GOSEC),github.com/securego/gosec/v2/cmd/gosec,$(GOSEC_VERSION))

$(GOVULNCHECK): $(call tool_stamp,govulncheck,$(GOVULNCHECK_VERSION)) | $(BIN)
	$(call go-install-tool,$(GOVULNCHECK),golang.org/x/vuln/cmd/govulncheck,$(GOVULNCHECK_VERSION))

$(OAPI_CODEGEN): $(call tool_stamp,oapi-codegen,$(OAPI_CODEGEN_VERSION)) | $(BIN)
	$(call go-install-tool,$(OAPI_CODEGEN),github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen,$(OAPI_CODEGEN_VERSION))

$(KUBECONFORM): $(call tool_stamp,kubeconform,$(KUBECONFORM_VERSION)) | $(BIN)
	$(call go-install-tool,$(KUBECONFORM),github.com/yannh/kubeconform/cmd/kubeconform,$(KUBECONFORM_VERSION))

$(KIND): $(call tool_stamp,kind,$(KIND_VERSION)) | $(BIN)
	$(call go-install-tool,$(KIND),sigs.k8s.io/kind,$(KIND_VERSION))

# Helm's release build stamps its version into the binary; without it `helm version` says v4.3.
$(HELM_BIN): $(call tool_stamp,helm,$(HELM_VERSION)) | $(BIN)
	$(call go-install-tool,$(HELM_BIN),helm.sh/helm/v4/cmd/helm,$(HELM_VERSION),-ldflags=-X=helm.sh/helm/v4/internal/version.version=$(HELM_VERSION))

# helm-unittest's go.mod replaces a dependency, which go install refuses; it is built inside its
# own module, as downloaded and checked against the checksum database, with that go.mod and go.sum.
$(HELM_UNITTEST): $(call tool_stamp,helm-unittest,$(HELM_UNITTEST_VERSION)) | $(BIN)
	@echo "Building helm-unittest $(HELM_UNITTEST_VERSION) from its module"
	@dir=$$(cd $(BACKEND_DIR) && go mod download -json github.com/helm-unittest/helm-unittest@$(HELM_UNITTEST_VERSION) | sed -n 's/^[[:space:]]*"Dir": "\(.*\)",$$/\1/p'); \
	  test -n "$$dir" || { echo "go mod download did not name the module's directory"; exit 1; }; \
	  cd "$$dir" && GOFLAGS=-mod=readonly go build -trimpath -o $(HELM_UNITTEST) ./cmd/helm-unittest

# go-install-tool 'go install's a package into bin/ under the versioned path its variable names.
# $1 - target path; $2 - package; $3 - version; $4 - extra go install flags. go install names the
# binary after the package, so it installs into a directory of its own, which then holds that one
# file, and the file is moved onto the versioned path. It runs in backend/, so the toolchain
# go.mod selects builds it.
define go-install-tool
@{ \
set -e; \
package=$(2)@$(3); \
echo "Building $${package}"; \
rm -rf $(1).install; \
cd $(BACKEND_DIR) && GOBIN=$(1).install go install -trimpath $(4) $${package}; \
mv $(1).install/* $(1); \
rm -rf $(1).install; \
touch $(1); \
}
endef
