# Include ODC common make targets
DEV_KIT_VERSION := v1.0.4
-include common.mk
common.mk:
	curl --fail -sSL https://raw.githubusercontent.com/opendefensecloud/dev-kit/$(DEV_KIT_VERSION)/common.mk -o common.mk.download && \
	mv common.mk.download $@

HACK_DIR ?= $(shell cd hack 2>/dev/null && pwd)
SOLAR_CHART_DIR ?= $(BUILD_PATH)/charts/solar

OCM_DEMO_DIR ?= $(BUILD_PATH)/test/fixtures/ocm-demo-ctf
OCM_DEMO_VERSION ?= v26.4.2

ENVTEST_K8S_VERSION ?= 1.34.1

export GOPRIVATE=*.go.opendefense.cloud/solar
export GNOSUMDB=*.go.opendefense.cloud/solar
export GNOPROXY=*.go.opendefense.cloud/solar

APISERVER_IMG ?= solar-apiserver:latest
MANAGER_IMG ?= solar-controller-manager:latest
RENDERER_IMG ?= solar-renderer:latest
DISCOVERY_IMG ?= solar-discovery:latest
DOCS_IMG ?= solar-docs:latest

TIMESTAMP := $(shell date '+%Y%m%d%H%M%S')
DEV_TAG ?= dev.$(TIMESTAMP)
export DEV_TAG

.PHONY: codegen
codegen: $(OPENAPI_GEN) manifests ## Run code generation, e.g. openapi
	OPENAPI_GEN=$(OPENAPI_GEN) ./hack/update-codegen.sh
	$(MAKE) docs-crd-ref

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

.PHONY: test
test: $(SETUP_ENVTEST) $(GINKGO) ocm-transfer-demo ## Run all tests
	OCM=$(OCM) \
	KUBEBUILDER_ASSETS="$(shell $(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" \
	$(GINKGO) -r -cover --fail-fast --require-suite -covermode count --output-dir=$(BUILD_PATH) -coverprofile=solar.full.coverprofile $(testargs)
	@grep -v /solar/api solar.full.coverprofile > solar.coverprofile

.PHONY: test-e2e
test-e2e: manifests ## Run the e2e tests. Expected an isolated environment using Kind.
	HELM=$(HELM) \
	KIND=$(KIND) \
	KIND_CLUSTER=$(KIND_CLUSTER_E2E) \
	KUBECTL=$(KUBECTL) \
	MAKE=$(MAKE) \
	OCM=$(OCM) \
	$(GO) test -tags=e2e ./test/e2e/ -v -ginkgo.v

.PHONY: manifests
manifests: $(CONTROLLER_GEN) ## Generate ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./pkg/controller/...;./api/..." output:rbac:artifacts:config=$(SOLAR_CHART_DIR)/files

.PHONY: kind-load-local-images
kind-load-local-images:
	$(KIND) load docker-image localhost/local/solar-apiserver:$(TAG) --name $(KIND_CLUSTER)
	$(KIND) load docker-image localhost/local/solar-controller-manager:$(TAG) --name $(KIND_CLUSTER)
	$(KIND) load docker-image localhost/local/solar-renderer:$(TAG) --name $(KIND_CLUSTER)
	$(KIND) load docker-image localhost/local/solar-discovery:$(TAG) --name $(KIND_CLUSTER)

KIND_CLUSTER_E2E ?= solar-test-e2e

.PHONY: e2e-cluster
e2e-cluster: ocm-transfer-demo ## Create a e2e test cluster (Contains everything as a dev-cluster except the solar-api itself)
	$(MAKE) setup-local-cluster KIND_CLUSTER=$(KIND_CLUSTER_E2E)
	$(MAKE) docker-build-local-images TAG=e2e
	$(MAKE) kind-load-local-images TAG=e2e KIND_CLUSTER=$(KIND_CLUSTER_E2E)
	TAG=e2e KIND_CLUSTER=$(KIND_CLUSTER_E2E) SKIP_SOLAR=true $(HACK_DIR)/dev-cluster.sh

.PHONY: cleanup-e2e-cluster
cleanup-e2e-cluster: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER_E2E)

KIND_CLUSTER_DEV ?= solar-dev

.PHONY: dev-cluster
dev-cluster: ocm-transfer-demo ## Create a kind cluster for local development / testing
	$(MAKE) setup-local-cluster KIND_CLUSTER=$(KIND_CLUSTER_DEV)
	$(MAKE) docker-build-local-images TAG=$(DEV_TAG)
	$(MAKE) kind-load-local-images TAG=$(DEV_TAG) KIND_CLUSTER=$(KIND_CLUSTER_DEV)
	TAG=$(DEV_TAG) KIND_CLUSTER=$(KIND_CLUSTER_DEV) $(HACK_DIR)/dev-cluster.sh

.PHONY: dev-cluster-rebuild
dev-cluster-rebuild: ## Rebuild images from source and load them into the local dev cluster
	$(MAKE) docker-build-local-images TAG=$(DEV_TAG)
	$(MAKE) kind-load-local-images TAG=$(DEV_TAG) KIND_CLUSTER=$(KIND_CLUSTER_DEV)
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

DOCKER ?= docker

.PHONY: docker-build
docker-build: docker-build-apiserver docker-build-manager docker-build-discovery docker-build-renderer

.PHONY: docker-build-local-images
docker-build-local-images:
	$(MAKE) \
		APISERVER_IMG=localhost/local/solar-apiserver:$(TAG) \
		MANAGER_IMG=localhost/local/solar-controller-manager:$(TAG) \
		RENDERER_IMG=localhost/local/solar-renderer:$(TAG) \
		DISCOVERY_IMG=localhost/local/solar-discovery:$(TAG) docker-build

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

.PHONY: docs-docker-build
docs-docker-build:
	@$(DOCKER) build -t ${DOCS_IMG} -f mkdocs.Dockerfile .

.PHONY: docs
docs: docs-docker-build ## Serve the documentation using Docker.
	@$(DOCKER) run --rm -it -p 8000:8000 -v ${PWD}:/docs ${DOCS_IMG}

.PHONY: docs-crd-ref
docs-crd-ref: $(CRD_REF_DOCS) ## Generate CRD reference documentation.
	$(CRD_REF_DOCS) --source-path=api/solar/v1alpha1 --config=crd-ref-docs.yaml --output-path=./docs/user-guide/api-reference.md --renderer=markdown

.PHONY: docs-helm-ref
docs-helm-ref: $(HELM_DOCS) ## Generate Helm Chart reference documentation.
	cd $(SOLAR_CHART_DIR) && $(HELM_DOCS) --template-files=README.md.gotmpl

.PHONY: ocm-transfer-demo
ocm-transfer-demo: $(OCM) ## Transfer the ocm-demo component to the local OCM CTF directory
	@if [ ! -d $(OCM_DEMO_DIR) ] || ! grep -q '"tag":"$(OCM_DEMO_VERSION)"' $(OCM_DEMO_DIR)/artifact-index.json 2>/dev/null; then \
		rm -rf $(OCM_DEMO_DIR); \
		$(OCM) transfer components --latest --copy-resources --type directory ghcr.io/opendefensecloud//opendefense.cloud/ocm-demo:$(OCM_DEMO_VERSION) $(OCM_DEMO_DIR); \
	fi
