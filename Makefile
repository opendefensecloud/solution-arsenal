# Include ODC common make targets
DEV_KIT_VERSION := v1.0.11
-include common.mk
common.mk:
	@[ -f .common.mk-download ] || \
		curl --fail -sSL https://raw.githubusercontent.com/opendefensecloud/dev-kit/$(DEV_KIT_VERSION)/common.mk \
		  -o .common.mk-download
	mv .common.mk-download $@
	printf '%s' '$(DEV_KIT_VERSION)' > .common.mk-version
	touch .common.mk-checked

HACK_DIR ?= $(shell cd hack 2>/dev/null && pwd)
SOLAR_CHART_DIR ?= $(BUILD_PATH)/charts/solar

OCM_DEMO_DIR ?= $(BUILD_PATH)/test/fixtures/ocm-demo-ctf
OCM_DEMO_VERSION ?= v26.4.2

ENVTEST_K8S_VERSION ?= 1.36.1

# Kind node image for e2e — defaults to track ENVTEST_K8S_VERSION so envtest
# (`make test`) and Kind-based e2e (`make test-e2e`) target the same K8s
# release. Override KIND_NODE_IMAGE directly if you need to decouple them.
# The patsubst tolerates both "1.36.0" and "v1.36.0" inputs.
KIND_NODE_IMAGE ?= kindest/node:v$(patsubst v%,%,$(ENVTEST_K8S_VERSION))

export CERTMANAGER_VERSION := v1.20.3
export TRUSTMANAGER_VERSION := v0.23.0
export ZOT_VERSION := 0.1.116

export GOPRIVATE=*.go.opendefense.cloud/solar
export GNOSUMDB=*.go.opendefense.cloud/solar
export GNOPROXY=*.go.opendefense.cloud/solar

# --- REGISTRY & TAG CONFIGURATION FOR CI INTEGRATION ---
REGISTRY           ?= localhost/local
TAG                ?= e2e
E2E_IMAGE_SOURCE   ?= local
KIND_CLUSTER_E2E   ?= solar-test-e2e
KIND_CLUSTER_DEV   ?= solar-dev

APISERVER_IMG ?= $(REGISTRY)/solar-apiserver:$(TAG)
MANAGER_IMG   ?= $(REGISTRY)/solar-controller-manager:$(TAG)
RENDERER_IMG  ?= $(REGISTRY)/solar-renderer:$(TAG)
DISCOVERY_IMG ?= $(REGISTRY)/solar-discovery:$(TAG)
UI_IMG        ?= $(REGISTRY)/solar-ui:$(TAG)
DOCS_IMG      ?= solar-docs:latest

TIMESTAMP := $(shell date '+%Y%m%d%H%M%S')
DEV_TAG ?= dev.$(TIMESTAMP)
export DEV_TAG

.PHONY: codegen
codegen: $(OPENAPI_GEN) manifests ## Run code generation, e.g. openapi
	OPENAPI_GEN=$(OPENAPI_GEN) ./hack/update-codegen.sh
	$(MAKE) docs-crd-ref
	$(MAKE) docs-helm-ref

.PHONY: fmt
fmt: $(ADDLICENSE) $(GOLANGCI_LINT) ## Add license headers and format code
	git ls-files | grep '.*\.go$$' | xargs $(ADDLICENSE) -c 'BWI GmbH and Solution Arsenal contributors' -l apache -s=only
	$(GO) fmt ./...
	$(GOLANGCI_LINT) run --fix

.PHONY: lint
lint: lint-no-golangci golangci-lint ## Run linters

.PHONY: lint-no-golangci
lint-no-golangci: $(ADDLICENSE) shellcheck  ## Run linters but not golangci-lint to exit early in CI/CD pipeline
	git ls-files | grep '.*\.go$$' | xargs $(ADDLICENSE) -check -l apache -s=only -check
	bash hack/check-crd-ref-docs-templates.sh

.PHONY: envtest-binaries-sideload
envtest-binaries-sideload: $(SETUP_ENVTEST)  ## Populate the envtest cache for ENVTEST_K8S_VERSION from upstream K8s/etcd releases when controller-tools hasn't packaged it. No-op if already cached. See hack/envtest-sideload.sh.
	@SETUP_ENVTEST=$(SETUP_ENVTEST) BIN_DIR=$(LOCALBIN) YQ=$(YQ) \
		bash hack/envtest-sideload.sh $(ENVTEST_K8S_VERSION)

.PHONY: test
test: $(SETUP_ENVTEST) $(GINKGO) envtest-binaries-sideload ocm-transfer-demo ## Run all tests
	mkdir -p $(BUILD_PATH)/coverdata
	OCM=$(OCM) \
	GOCOVERDIR=$(BUILD_PATH)/coverdata \
	KUBEBUILDER_ASSETS="$(shell $(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -i -p path)" \
	$(GINKGO) -r -cover --fail-fast --require-suite -covermode count --output-dir=$(BUILD_PATH) -coverprofile=solar.full.coverprofile --keep-separate-coverprofiles $(testargs)
	@echo 'mode: count' > $(BUILD_PATH)/solar.full.coverprofile && \
	 cat $(BUILD_PATH)/*_solar.full.coverprofile 2>/dev/null | grep -v '^mode:' >> $(BUILD_PATH)/solar.full.coverprofile || true
	@grep -v 'zz_generated' $(BUILD_PATH)/solar.full.coverprofile > solar.coverprofile || true

.PHONY: test-e2e
test-e2e: manifests ## Run the e2e tests. Expected an isolated environment using Kind.
	E2E_IMAGE_SOURCE=$(E2E_IMAGE_SOURCE) \
	HELM=$(HELM) \
	KIND=$(KIND) \
	KIND_CLUSTER=$(KIND_CLUSTER_E2E) \
	KUBECTL=$(KUBECTL) \
	MAKE=$(MAKE) \
	IMAGE_TAG=$(TAG) \
	OCM=$(OCM) \
	REGISTRY=$(REGISTRY) \
	$(GO) test -count=1 -tags=e2e -timeout 15m ./test/e2e/ -v -ginkgo.v


.PHONY: manifests
manifests: $(CONTROLLER_GEN) ## Generate ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./pkg/controller/...;./api/..." output:rbac:artifacts:config=$(SOLAR_CHART_DIR)/files

.PHONY: kind-load-local-images
kind-load-local-images:
	@KIND=$(KIND) bash $(HACK_DIR)/require-kind-version.sh
	$(KIND) load docker-image $(APISERVER_IMG) --name $(KIND_CLUSTER)
	$(KIND) load docker-image $(MANAGER_IMG) --name $(KIND_CLUSTER)
	$(KIND) load docker-image $(RENDERER_IMG) --name $(KIND_CLUSTER)
	$(KIND) load docker-image $(DISCOVERY_IMG) --name $(KIND_CLUSTER)
	$(KIND) load docker-image $(UI_IMG) --name $(KIND_CLUSTER)

.PHONY: e2e-cluster
e2e-cluster: ocm-transfer-demo ## Create a e2e test cluster (Contains everything as a dev-cluster except the solar-api itself). Pin K8s via KIND_NODE_IMAGE (defaults from ENVTEST_K8S_VERSION). Pass KIND_RECREATE=1 to delete + recreate on image mismatch.
	@KIND=$(KIND) DOCKER=$(DOCKER) KIND_RECREATE=$(KIND_RECREATE) \
		bash hack/ensure-kind-cluster.sh $(KIND_CLUSTER_E2E) $(KIND_NODE_IMAGE)
	$(MAKE) setup-local-cluster KIND_CLUSTER=$(KIND_CLUSTER_E2E)
	@if [ "$(E2E_IMAGE_SOURCE)" = "local" ]; then \
		$(MAKE) docker-build-local-images TAG=e2e REGISTRY=$(REGISTRY); \
		$(MAKE) kind-load-local-images TAG=e2e KIND_CLUSTER=$(KIND_CLUSTER_E2E) REGISTRY=$(REGISTRY); \
	fi
	REGISTRY=$(REGISTRY) TAG=$(TAG) KIND_CLUSTER=$(KIND_CLUSTER_E2E) SKIP_SOLAR=true $(HACK_DIR)/dev-cluster.sh

.PHONY: cleanup-e2e-cluster
cleanup-e2e-cluster: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER_E2E)

.PHONY: dev-cluster
dev-cluster: ocm-transfer-demo ## Create a kind cluster for local development / testing. Pin K8s via KIND_NODE_IMAGE (defaults from ENVTEST_K8S_VERSION). Pass KIND_RECREATE=1 to delete + recreate on image mismatch.
	@KIND=$(KIND) DOCKER=$(DOCKER) KIND_RECREATE=$(KIND_RECREATE) \
		bash hack/ensure-kind-cluster.sh $(KIND_CLUSTER_DEV) $(KIND_NODE_IMAGE)
	$(MAKE) setup-local-cluster KIND_CLUSTER=$(KIND_CLUSTER_DEV)
	$(MAKE) docker-build-local-images TAG=$(DEV_TAG)
	$(MAKE) kind-load-local-images TAG=$(DEV_TAG) KIND_CLUSTER=$(KIND_CLUSTER_DEV)
	REGISTRY=$(REGISTRY) TAG=$(DEV_TAG) KIND_CLUSTER=$(KIND_CLUSTER_DEV) $(HACK_DIR)/dev-cluster.sh

.PHONY: dev-cluster-rebuild
dev-cluster-rebuild: ## Rebuild images from source and load them into the local dev cluster
	$(MAKE) docker-build-local-images TAG=$(DEV_TAG) REGISTRY=$(REGISTRY)
	$(MAKE) kind-load-local-images TAG=$(DEV_TAG) KIND_CLUSTER=$(KIND_CLUSTER_DEV) REGISTRY=$(REGISTRY)
	$(HELM) upgrade --namespace solar-system solar charts/solar \
		-f test/fixtures/solar.values.yaml \
		--set apiserver.image.tag=$(DEV_TAG) \
		--set controller.image.tag=$(DEV_TAG) \
		--set renderer.image.tag=$(DEV_TAG)
	$(HELM) upgrade --install --namespace solar-system solar-discovery charts/solar-discovery \
		-f test/fixtures/solar-discovery-webhook.values.yaml \
		--set image.tag=$(DEV_TAG) \
		--set namespace=solar-system

.PHONY: cleanup-dev-cluster
cleanup-dev-cluster: ## Tear down the Kind cluster used for local tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER_DEV)

.PHONY: cleanup-all-clusters
cleanup-all-clusters: ## Tear down all SolAr Kind clusters
	@for cluster in $$($(KIND) get clusters 2>/dev/null); do \
		case "$$cluster" in \
			$(KIND_CLUSTER_DEV)|$(KIND_CLUSTER_E2E)|$(KIND_CLUSTER_UI_DEV)|$(KIND_CLUSTER_UI_E2E)) \
				echo "Deleting Kind cluster '$$cluster'..."; \
				$(KIND) delete cluster --name "$$cluster" ;; \
		esac; \
	done

DOCKER ?= docker

.PHONY: docker-build
docker-build: docker-build-apiserver docker-build-manager docker-build-discovery docker-build-renderer docker-build-ui

.PHONY: docker-build-local-images
docker-build-local-images:
	$(MAKE) docker-build

.PHONY: docker-build-apiserver
docker-build-apiserver:
	$(DOCKER) build --target apiserver -t ${APISERVER_IMG} .

.PHONY: docker-build-manager
docker-build-manager:
	$(DOCKER) build --target manager -t ${MANAGER_IMG} .

.PHONY: docker-build-discovery
docker-build-discovery:
	$(DOCKER) build --target discovery -t ${DISCOVERY_IMG} .

.PHONY: docker-build-renderer
docker-build-renderer:
	$(DOCKER) build --target renderer -t ${RENDERER_IMG} .

.PHONY: docker-build-ui
docker-build-ui:
	$(DOCKER) build --target ui -t ${UI_IMG} .

##@ UI

PNPM ?= pnpm
KIND_CLUSTER_UI_DEV ?= solar-ui-dev
KIND_CLUSTER_UI_E2E ?= solar-test-e2e-ui

.PHONY: ui-install
ui-install: ## Install frontend dependencies
	cd web && $(PNPM) install

.PHONY: ui-build
ui-build: ui-install ## Build the frontend for production
	cd web && $(PNPM) build
	rm -rf pkg/ui/static
	cp -r web/dist pkg/ui/static
	touch pkg/ui/static/.gitkeep

.PHONY: ui-lint
ui-lint: ## Lint frontend code
	cd web && $(PNPM) lint

.PHONY: ui-dev-cluster
ui-dev-cluster: ocm-transfer-demo ## Create a Kind cluster with SolAr + Dex for UI development
	$(HACK_DIR)/generate-dex-certs.sh
	KIND_CONFIG=test/fixtures/e2e/kind-config-oidc.yaml $(MAKE) setup-local-cluster KIND_CLUSTER=$(KIND_CLUSTER_UI_DEV)
	$(MAKE) docker-build-local-images TAG=$(DEV_TAG)
	$(MAKE) kind-load-local-images TAG=$(DEV_TAG) KIND_CLUSTER=$(KIND_CLUSTER_UI_DEV)
	TAG=$(DEV_TAG) KIND_CLUSTER=$(KIND_CLUSTER_UI_DEV) $(HACK_DIR)/dev-cluster.sh
	KIND_CLUSTER=$(KIND_CLUSTER_UI_DEV) $(HACK_DIR)/setup-dex.sh

.PHONY: ui-cleanup-dev-cluster
ui-cleanup-dev-cluster: ## Tear down the UI dev cluster
	@$(KIND) delete cluster --name $(KIND_CLUSTER_UI_DEV)

.PHONY: ui-seed-data
ui-seed-data: ## Seed demo resources (targets, releases, components, etc.) into the cluster
	$(HACK_DIR)/seed-demo-data.sh

.PHONY: ui-dev
ui-dev: ui-install ## Start Go backend + Vite dev server against the UI dev cluster
	@case "$$($(KIND) get clusters 2>/dev/null)" in \
		*"$(KIND_CLUSTER_UI_DEV)"*) ;; \
		*) echo "UI dev cluster not found. Creating it..."; $(MAKE) ui-dev-cluster ;; \
	esac
	@test -f test/fixtures/dex-ca.crt || { echo "Dex CA cert not found. Run 'make ui-dev-cluster' first."; exit 1; }
	@echo "Starting Dex port-forward + Vite dev server + solar-ui backend..."
	@echo "Open http://localhost:8090 in your browser."
	@echo ""
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER_UI_DEV) > /tmp/solar-ui-dev-kubeconfig
	cd web && $(PNPM) exec concurrently --kill-others --names "dex,vite,bff" --prefix-colors "magenta,cyan,yellow" \
		"KUBECONFIG=/tmp/solar-ui-dev-kubeconfig $(KUBECTL) port-forward -n dex service/dex 5556:5556" \
		"$(PNPM) dev --port 5173" \
		"sleep 2 && cd $(BUILD_PATH) && $(GO) run ./cmd/solar-ui \
			--listen=0.0.0.0:8090 \
			--kubeconfig=/tmp/solar-ui-dev-kubeconfig \
			--oidc-issuer=https://localhost:5556 \
			--oidc-ca-cert=$(BUILD_PATH)/test/fixtures/dex-ca.crt \
			--oidc-client-id=solar-ui \
			--oidc-client-secret=solar-ui-secret \
			--oidc-redirect-url=http://localhost:8090/api/auth/callback \
			--auth-mode=token \
			--dev-vite-url=http://localhost:5173"

.PHONY: ui-e2e-cluster
ui-e2e-cluster: ocm-transfer-demo ## Create a Kind cluster with Dex + SolAr for UI e2e testing
	$(HACK_DIR)/generate-dex-certs.sh
	KIND_CONFIG=test/fixtures/e2e/kind-config-oidc.yaml $(MAKE) setup-local-cluster KIND_CLUSTER=$(KIND_CLUSTER_UI_E2E)
	$(MAKE) docker-build-local-images TAG=e2e
	$(MAKE) kind-load-local-images TAG=e2e KIND_CLUSTER=$(KIND_CLUSTER_UI_E2E)
	TAG=e2e KIND_CLUSTER=$(KIND_CLUSTER_UI_E2E) $(HACK_DIR)/dev-cluster.sh
	KIND_CLUSTER=$(KIND_CLUSTER_UI_E2E) $(HACK_DIR)/setup-dex.sh

.PHONY: ui-cleanup-e2e-cluster
ui-cleanup-e2e-cluster: ## Tear down the UI e2e cluster
	@$(KIND) delete cluster --name $(KIND_CLUSTER_UI_E2E)

.PHONY: ui-test-e2e
ui-test-e2e: ui-build ## Run Playwright UI e2e tests (auto-creates cluster if needed)
	@case "$$($(KIND) get clusters 2>/dev/null)" in \
		*"$(KIND_CLUSTER_UI_E2E)"*) ;; \
		*) echo "UI e2e cluster not found. Creating it..."; $(MAKE) ui-e2e-cluster ;; \
	esac
	@# Build and run the compiled binary directly rather than `go run`: `go run`
	@# spawns a child process that outlives a `kill` of its parent, leaving an
	@# orphaned backend bound to :8090 that breaks (and flakes) subsequent runs.
	@$(GO) build -o $(LOCALBIN)/solar-ui ./cmd/solar-ui
	@$(KIND) get kubeconfig --name $(KIND_CLUSTER_UI_E2E) > /tmp/solar-e2e-ui-kubeconfig
	@echo "Starting Dex port-forward for e2e tests..."
	@KUBECONFIG=/tmp/solar-e2e-ui-kubeconfig $(KUBECTL) port-forward -n dex service/dex 5556:5556 >/tmp/solar-e2e-dex-pf.log 2>&1 & \
	PF_PID=$$!; \
	trap 'kill $$PF_PID $$UI_PID 2>/dev/null; wait $$PF_PID $$UI_PID 2>/dev/null' EXIT INT TERM; \
	echo "Waiting for Dex (https://localhost:5556)..."; \
	for i in $$(seq 1 60); do \
		curl -skf https://localhost:5556/.well-known/openid-configuration >/dev/null 2>&1 && break; \
		sleep 1; \
	done; \
	echo "Starting solar-ui backend..."; \
	$(LOCALBIN)/solar-ui \
		--listen=0.0.0.0:8090 \
		--kubeconfig=/tmp/solar-e2e-ui-kubeconfig \
		--oidc-issuer=https://localhost:5556 \
		--oidc-ca-cert=$(BUILD_PATH)/test/fixtures/dex-ca.crt \
		--oidc-client-id=solar-ui \
		--oidc-client-secret=solar-ui-secret \
		--oidc-redirect-url=http://localhost:8090/api/auth/callback \
		--auth-mode=token >/tmp/solar-e2e-bff.log 2>&1 & \
	UI_PID=$$!; \
	echo "Waiting for solar-ui backend (http://localhost:8090)..."; \
	for i in $$(seq 1 60); do \
		curl -sf http://localhost:8090/api/auth/me >/dev/null 2>&1 && break; \
		sleep 1; \
	done; \
	cd web && DEX_LOCAL_PORT=5556 $(PNPM) exec playwright test; \
	exit $$?
ifeq ($(OS),darwin)
ui-test-e2e: ui-playwright-browser

.PHONY: ui-playwright-browser
ui-playwright-browser: ui-install ## Install Playwright's Chromium browser (macOS; Linux uses the Nix-provided chromium)
	cd web && $(PNPM) exec playwright install chromium
endif

##@ Docs

.PHONY: docs-docker-build
docs-docker-build:
	@$(DOCKER) build -t ${DOCS_IMG} -f mkdocs.Dockerfile .

.PHONY: docs
docs: docs-docker-build ## Serve the documentation using Docker.
	@$(DOCKER) run --rm -it -p 8000:8000 -v ${PWD}:/docs ${DOCS_IMG}

.PHONY: docs-crd-ref
docs-crd-ref: $(CRD_REF_DOCS) ## Generate CRD reference documentation.
	$(CRD_REF_DOCS) --source-path=api/solar/v1alpha1 --config=crd-ref-docs.yaml --output-path=./docs/user-guide/api-reference.md --renderer=markdown --templates-dir=hack/crd-ref-docs-templates

.PHONY: docs-helm-ref
docs-helm-ref: $(HELM_DOCS) ## Generate Helm Chart reference documentation.
	cd $(SOLAR_CHART_DIR) && $(HELM_DOCS) --template-files=README.md.gotmpl

.PHONY: ocm-transfer-demo
ocm-transfer-demo: $(OCM) ## Transfer the ocm-demo component to the local OCM CTF directory
	@if [ ! -d $(OCM_DEMO_DIR) ] || ! grep -q '"tag":"$(OCM_DEMO_VERSION)"' $(OCM_DEMO_DIR)/artifact-index.json 2>/dev/null; then \
		rm -rf $(OCM_DEMO_DIR); \
		$(OCM) transfer components --latest --copy-resources --type directory ghcr.io/opendefensecloud//opendefense.cloud/ocm-demo:$(OCM_DEMO_VERSION) $(OCM_DEMO_DIR); \
	fi
