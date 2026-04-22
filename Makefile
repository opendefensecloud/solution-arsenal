
# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Set MAKEFLAGS to suppress entering/leaving directory messages
MAKEFLAGS += --no-print-directory

BUILD_PATH ?= $(shell pwd)
HACK_DIR ?= $(shell cd hack 2>/dev/null && pwd)
LOCALBIN ?= $(BUILD_PATH)/bin
SOLAR_CHART_DIR ?= $(BUILD_PATH)/charts/solar
OCM_DEMO_DIR ?= $(BUILD_PATH)/test/fixtures/ocm-demo-ctf
OCM_DEMO_VERSION ?= v26.4.1

OS := $(shell go env GOOS)
ARCH := $(shell go env GOARCH)

GO ?= go
DOCKER ?= docker
KIND ?= kind
HELM ?= helm

OSV_SCANNER ?= $(GO) tool github.com/google/osv-scanner/v2/cmd/osv-scanner
GINKGO ?= $(GO) tool github.com/onsi/ginkgo/v2/ginkgo
GOLANGCI_LINT ?= $(GO) tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint
SETUP_ENVTEST ?= $(GO) tool sigs.k8s.io/controller-runtime/tools/setup-envtest
ADDLICENSE ?= $(GO) tool github.com/google/addlicense
CONTROLLER_GEN ?= $(GO) tool sigs.k8s.io/controller-tools/cmd/controller-gen
OPENAPI_GEN ?= $(GO) tool k8s.io/kube-openapi/cmd/openapi-gen
CRD_REF_DOCS ?= $(GO) tool github.com/elastic/crd-ref-docs
HELM_DOCS ?= $(GO) tool github.com/norwoodj/helm-docs/cmd/helm-docs
OCM ?= $(GO) tool ocm.software/ocm/cmds/ocm

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
codegen: manifests ## Run code generation, e.g. openapi
	OPENAPI_GEN=$(OPENAPI_GEN) ./hack/update-codegen.sh
	$(MAKE) docs-crd-ref

.PHONY: fmt
fmt: ## Add license headers and format code
	find . -not -path '*/.*' -name '*.go' -exec $(ADDLICENSE) -c 'BWI GmbH and Solution Arsenal contributors' -l apache -s=only {} +
	$(GO) fmt ./...
	$(GOLANGCI_LINT) run --fix

.PHONY: mod
mod: ## Do go mod tidy, download, verify
	@$(GO) mod tidy
	@$(GO) mod download
	@$(GO) mod verify

.PHONY: lint
lint: lint-no-golangci ## Run linters such as golangci-lint and addlicence checks
	$(GOLANGCI_LINT) run -v

.PHONY: lint-no-golangci
lint-no-golangci:
	find . -not -path '*/.*' -name '*.go' -exec $(ADDLICENSE) -check  -l apache -s=only -check {} +
	shellcheck hack/*.sh

.PHONY: scan
scan:
	$(OSV_SCANNER) scan --config ./.osv-scanner.toml -r .

.PHONY: test
test: ocm-transfer-demo ## Run all tests
	@KUBEBUILDER_ASSETS="$(shell $(SETUP_ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" $(GINKGO) -r -cover --fail-fast --require-suite -covermode count --output-dir=$(BUILD_PATH) -coverprofile=solar.full.coverprofile $(testargs)
	@cat solar.full.coverprofile | grep -v /solar/api > solar.coverprofile

.PHONY: test-e2e
test-e2e: manifests ## Run the e2e tests. Expected an isolated environment using Kind.
	OCM="$(OCM)" KIND_CLUSTER=$(KIND_CLUSTER_E2E) go test -tags=e2e ./test/e2e/ -v -ginkgo.v

.PHONY: manifests
manifests: ## Generate ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./pkg/controller/...;./api/..." output:rbac:artifacts:config=$(SOLAR_CHART_DIR)/files

.PHONY: kind-load-local-images
kind-load-local-images:
	$(KIND) load docker-image localhost/local/solar-apiserver:$(TAG) --name $(KIND_CLUSTER)
	$(KIND) load docker-image localhost/local/solar-controller-manager:$(TAG) --name $(KIND_CLUSTER)
	$(KIND) load docker-image localhost/local/solar-renderer:$(TAG) --name $(KIND_CLUSTER)
	$(KIND) load docker-image localhost/local/solar-discovery:$(TAG) --name $(KIND_CLUSTER)

.PHONY: setup-local-cluster
setup-local-cluster: ## Set up a Kind cluster for local development if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

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
docs: docs-crd-ref docs-helm-ref docs-docker-build ## Serve the documentation using Docker.
	@$(DOCKER) run --rm -it -p 8000:8000 -v ${PWD}:/docs ${DOCS_IMG}

.PHONY: docs-crd-ref
docs-crd-ref: ## Generate CRD reference documentation.
	$(CRD_REF_DOCS) --source-path=api/solar/v1alpha1 --config=crd-ref-docs.yaml --output-path=./docs/user-guide/api-reference.md --renderer=markdown

.PHONY: docs-helm-ref
docs-helm-ref: ## Generate Helm Chart reference documentation.
	cd $(SOLAR_CHART_DIR) && $(HELM_DOCS) --template-files=README.md.gotmpl

.PHONY: ocm-transfer-demo
ocm-transfer-demo: ## Transfer the ocm-demo component to the local OCM CTF directory
	@if [ ! -d $(OCM_DEMO_DIR) ] || ! grep -q '"tag":"$(OCM_DEMO_VERSION)"' $(OCM_DEMO_DIR)/artifact-index.json 2>/dev/null; then \
		rm -rf $(OCM_DEMO_DIR); \
		$(OCM) transfer components --latest --copy-resources --type directory ghcr.io/opendefensecloud//opendefense.cloud/ocm-demo:$(OCM_DEMO_VERSION) $(OCM_DEMO_DIR); \
	fi
