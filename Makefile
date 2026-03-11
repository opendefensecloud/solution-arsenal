
# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Set MAKEFLAGS to suppress entering/leaving directory messages
MAKEFLAGS += --no-print-directory

BUILD_PATH ?= $(shell pwd)
HACK_DIR ?= $(shell cd hack 2>/dev/null && pwd)
LOCALBIN ?= $(BUILD_PATH)/bin
HELMDEMO_DIR ?= $(BUILD_PATH)/test/fixtures/helmdemo-ctf

OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)

GO ?= go
SHELLCHECK ?= shellcheck
OSV_SCANNER ?= osv-scanner
MKDOCS ?= mkdocs
DOCKER ?= docker
KIND ?= kind
KUBECTL ?= kubectl
HELM ?= helm
YQ ?= yq
GINKGO ?= $(LOCALBIN)/ginkgo
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
SETUP_ENVTEST ?= $(LOCALBIN)/setup-envtest
ADDLICENSE ?= $(LOCALBIN)/addlicense
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
OPENAPI_GEN ?= $(LOCALBIN)/openapi-gen
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs
OCM ?= $(LOCALBIN)/ocm

GINKGO_VERSION ?= $(shell go list -json -m -u github.com/onsi/ginkgo/v2 | jq -r '.Version')
GOLANGCI_LINT_VERSION ?= v2.10.1
SETUP_ENVTEST_VERSION ?= release-0.22
ADDLICENSE_VERSION ?= v1.1.1
CONTROLLER_TOOLS_VERSION ?= v0.19.0
OPENAPI_GEN_VERSION ?= $(shell go list -json -m -u k8s.io/kube-openapi | jq -r '.Version')
ENVTEST_K8S_VERSION ?= 1.34.1
CRD_REF_DOCS_VERSION ?= v0.2.0
OCM_VERSION ?= 0.34.3

export GOPRIVATE=*.go.opendefense.cloud/solar
export GNOSUMDB=*.go.opendefense.cloud/solar
export GNOPROXY=*.go.opendefense.cloud/solar

APISERVER_IMG ?= solar-apiserver:latest
MANAGER_IMG ?= solar-controller-manager:latest
RENDERER_IMG ?= solar-renderer:latest
DISCOVERY_WORKER_IMG ?= solar-discovery-worker:latest
DOCS_IMG ?= solar-docs:latest

TIMESTAMP := $(shell date '+%Y%m%d%H%M%S')
DEV_TAG ?= dev.$(TIMESTAMP)
export DEV_TAG

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: clean
clean:
	rm -rf $(LOCALBIN)

.PHONY: codegen
codegen: openapi-gen manifests ## Run code generation, e.g. openapi
	OPENAPI_GEN=$(OPENAPI_GEN) ./hack/update-codegen.sh
	$(MAKE) docs-crd-ref

.PHONY: fmt
fmt: addlicense golangci-lint ## Add license headers and format code
	find . -not -path '*/.*' -name '*.go' -exec $(ADDLICENSE) -c 'BWI GmbH and Solution Arsenal contributors' -l apache -s=only {} +
	$(GO) fmt ./...
	$(GOLANGCI_LINT) run --fix

.PHONY: mod
mod: ## Do go mod tidy, download, verify
	@$(GO) mod tidy
	@$(GO) mod download
	@$(GO) mod verify

.PHONY: lint
lint: lint-no-golangci golangci-lint ## Run linters such as golangci-lint and addlicence checks
	$(GOLANGCI_LINT) run -v

.PHONY: lint-no-golangci
lint-no-golangci: addlicense
	find . -not -path '*/.*' -name '*.go' -exec $(ADDLICENSE) -check  -l apache -s=only -check {} +
	shellcheck hack/*.sh

.PHONY: scan
scan:
	$(OSV_SCANNER) scan --config ./.osv-scanner.toml -r .

.PHONY: test
test: setup-envtest ginkgo ocm-transfer-helmdemo ## Run all tests
	@KUBEBUILDER_ASSETS="$(shell $(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" $(GINKGO) -r -cover --fail-fast --require-suite -covermode count --output-dir=$(BUILD_PATH) -coverprofile=solar.full.coverprofile $(testargs)
	@cat solar.full.coverprofile | grep -v /solar/api > solar.coverprofile

.PHONY: manifests
manifests: controller-gen ## Generate ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./pkg/controller/...;./api/..." output:rbac:artifacts:config=charts/solar/files


KIND_CLUSTER_E2E ?= solar-test-e2e

.PHONY: setup-test-e2e
setup-test-e2e: ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER_E2E)"*) \
			echo "Kind cluster '$(KIND_CLUSTER_E2E)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER_E2E)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER_E2E) ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e manifests ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND=$(KIND) KIND_CLUSTER=$(KIND_CLUSTER_E2E) HELM=$(HELM) go test -tags=e2e ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER_E2E)

.PHONY: kind-load-dev-images
kind-load-dev-images: docker-build-dev-images
	$(KIND) load docker-image localhost/local/solar-apiserver:$(DEV_TAG) --name $(KIND_CLUSTER_DEV)
	$(KIND) load docker-image localhost/local/solar-controller-manager:$(DEV_TAG) --name $(KIND_CLUSTER_DEV)
	$(KIND) load docker-image localhost/local/solar-renderer:$(DEV_TAG) --name $(KIND_CLUSTER_DEV)
	$(KIND) load docker-image localhost/local/solar-discovery-worker:$(DEV_TAG) --name $(KIND_CLUSTER_DEV)

KIND_CLUSTER_DEV ?= solar-dev

.PHONY: setup-dev-cluster
setup-dev-cluster: ## Set up a Kind cluster for local development if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER_DEV)"*) \
			echo "Kind cluster '$(KIND_CLUSTER_DEV)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER_DEV)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER_DEV) ;; \
	esac

.PHONY: dev-cluster
dev-cluster: setup-dev-cluster ocm-transfer-helmdemo kind-load-dev-images
	$(HACK_DIR)/dev-cluster.sh

.PHONY: dev-cluster-rebuild
dev-cluster-rebuild: kind-load-dev-images
	$(HELM) upgrade --namespace solar-system solar charts/solar \
		-f test/fixtures/solar.values.yaml \
		--set apiserver.image.tag=$(DEV_TAG) \
		--set controller.image.tag=$(DEV_TAG) \
		--set renderer.image.tag=$(DEV_TAG) \
		--set discovery.image.tag=$(DEV_TAG)

.PHONY: cleanup-dev-cluster
cleanup-dev-cluster: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER_DEV)

.PHONY: docker-build
docker-build: docker-build-apiserver docker-build-manager docker-build-discovery-worker docker-build-renderer

.PHONY: docker-build-dev-images
docker-build-dev-images:
	$(MAKE) \
		APISERVER_IMG=localhost/local/solar-apiserver:$(DEV_TAG) \
		MANAGER_IMG=localhost/local/solar-controller-manager:$(DEV_TAG) \
		RENDERER_IMG=localhost/local/solar-renderer:$(DEV_TAG) \
		DISCOVERY_WORKER_IMG=localhost/local/solar-discovery-worker:$(DEV_TAG) docker-build

.PHONY: docker-build-apiserver
docker-build-apiserver:
	$(DOCKER) build --target apiserver -t ${APISERVER_IMG} .

.PHONY: docker-build-manager
docker-build-manager:
	$(DOCKER) build --target manager -t ${MANAGER_IMG} .

.PHONY: docker-build-discovery-worker
docker-build-discovery-worker:
	$(DOCKER) build --target discovery-worker -t ${DISCOVERY_WORKER_IMG} .

.PHONY: docker-build-renderer
docker-build-renderer:
	$(DOCKER) build --target renderer -t ${RENDERER_IMG} .

.PHONY: docs-docker-build
docs-docker-build:
	@$(DOCKER) build -t ${DOCS_IMG} -f mkdocs.Dockerfile .

docs-crd-ref: crd-ref-docs ## Generate CRD reference documentation.
	$(CRD_REF_DOCS) --source-path=api/solar/v1alpha1 --config=crd-ref-docs.yaml --output-path=./docs/user-guide/api-reference.md --renderer=markdown

.PHONY: docs
docs: docs-docker-build ## Serve the documentation using Docker.
	@$(DOCKER) run --rm -it -p 8000:8000 -v ${PWD}:/docs squidfunk/mkdocs-material

.PHONY: ocm-transfer-helmdemo
ocm-transfer-helmdemo: ocm ## Transfer the helmdemo chart to the OCM charts repository
	@test -d $(HELMDEMO_DIR) || \
	$(OCM) transfer components --latest --copy-resources --type directory ghcr.io/open-component-model/ocm//ocm.software/toi/demo/helmdemo:0.12.0 $(HELMDEMO_DIR)

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: controller-gen
controller-gen: $(LOCALBIN) ## Download controller-gen locally if necessary.
	@test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: golangci-lint
golangci-lint: $(LOCALBIN) ## Download golangci-lint locally if necessary.
	@test -s $(LOCALBIN)/golangci-lint && $(LOCALBIN)/golangci-lint --version | grep -q $(GOLANGCI_LINT_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: ginkgo
ginkgo: $(LOCALBIN) ## Download ginkgo locally if necessary.
	@test -s $(LOCALBIN)/ginkgo && $(LOCALBIN)/ginkgo version | grep -q $(subst v,,$(GINKGO_VERSION)) || \
	GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: addlicense
addlicense: $(LOCALBIN) ## Download addlicense locally if necessary.
	@test -s $(LOCALBIN)/addlicense && grep -q $(ADDLICENSE_VERSION) $(LOCALBIN)/.addlicense-version 2>/dev/null || \
	GOBIN=$(LOCALBIN) go install github.com/google/addlicense@$(ADDLICENSE_VERSION); \
	echo $(ADDLICENSE_VERSION) > $(LOCALBIN)/.addlicense-version

.PHONY: setup-envtest
setup-envtest: $(LOCALBIN) ## Download setup-envtest locally if necessary.
	@test -s $(LOCALBIN)/setup-envtest && grep -q $(SETUP_ENVTEST_VERSION) $(LOCALBIN)/.setup-envtest-version 2>/dev/null || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION); \
	echo $(SETUP_ENVTEST_VERSION) > $(LOCALBIN)/.setup-envtest-version

.PHONY: openapi-gen
openapi-gen: $(LOCALBIN) ## Download openapi-gen locally if necessary.
	@test -s $(LOCALBIN)/openapi-gen && grep -q $(OPENAPI_GEN_VERSION) $(LOCALBIN)/.openapi-gen-version 2>/dev/null || \
	GOBIN=$(LOCALBIN) go install k8s.io/kube-openapi/cmd/openapi-gen@$(OPENAPI_GEN_VERSION); \
	echo $(OPENAPI_GEN_VERSION) > $(LOCALBIN)/.openapi-gen-version

.PHONY: crd-ref-docs
crd-ref-docs: $(LOCALBIN) ## Download crd-ref-docs locally if necessary.
	@test -s $(LOCALBIN)/crd-ref-docs && $(LOCALBIN)/crd-ref-docs --version | grep -q $(CRD_REF_DOCS_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/elastic/crd-ref-docs@$(CRD_REF_DOCS_VERSION)

.PHONY: ocm
ocm: $(LOCALBIN) ## Download ocm locally if necessary.
	@test -s $(LOCALBIN)/ocm && $(LOCALBIN)/ocm version | jq -r '"\(.Major).\(.Minor).\(.Patch)"' | grep -q $(OCM_VERSION) || (\
	curl -L -o $(LOCALBIN)/ocm.tar.gz "https://github.com/open-component-model/ocm/releases/download/v$(OCM_VERSION)/ocm-$(OCM_VERSION)-$(OS)-$(ARCH).tar.gz"; \
	tar -xvf $(LOCALBIN)/ocm.tar.gz -C $(LOCALBIN); \
	chmod +x $(LOCALBIN)/ocm; \
	rm $(LOCALBIN)/ocm.tar.gz)
