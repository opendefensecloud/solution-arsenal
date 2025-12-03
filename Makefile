# Solution Arsenal (SolAr) Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Binary names
BINARY_INDEX=solar-index
BINARY_DISCOVERY=solar-discovery
BINARY_RENDERER=solar-renderer
BINARY_AGENT=solar-agent

# Directories
CMD_DIR=cmd
BIN_DIR=bin
PKG_DIR=pkg
INTERNAL_DIR=internal

# Build info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

# Tools
GOLANGCI_LINT_VERSION=v1.59.1
CONTROLLER_GEN_VERSION=v0.16.5
LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

.PHONY: all build build-index build-discovery build-renderer build-agent \
        test test-unit test-integration test-e2e \
        lint fmt vet \
        generate clean help \
        tools install-golangci-lint install-controller-gen

##@ General

all: fmt vet lint test build ## Run all checks and build

help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

fmt: ## Run go fmt against code
	$(GOFMT) -s -w .

vet: ## Run go vet against code
	$(GOVET) ./...

lint: $(GOLANGCI_LINT) ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

generate: $(CONTROLLER_GEN) ## Generate code (deepcopy, CRDs, etc.)
	$(GOCMD) generate ./...
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/..."
	$(CONTROLLER_GEN) crd paths="./pkg/apis/..." output:crd:artifacts:config=config/crd/bases

manifests: $(CONTROLLER_GEN) ## Generate CRD manifests
	$(CONTROLLER_GEN) crd paths="./pkg/apis/..." output:crd:artifacts:config=config/crd/bases

##@ Testing

test: test-unit ## Run all tests

test-unit: ## Run unit tests
	$(GOTEST) -race -coverprofile=coverage.out -covermode=atomic ./...

test-integration: ## Run integration tests
	$(GOTEST) -race -tags=integration ./test/integration/...

test-e2e: ## Run end-to-end tests
	$(GOTEST) -race -tags=e2e ./test/e2e/...

coverage: test-unit ## Generate coverage report
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

##@ Build

build: build-index build-discovery build-renderer build-agent ## Build all binaries

build-index: ## Build solar-index binary
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_INDEX) ./$(CMD_DIR)/$(BINARY_INDEX)

build-discovery: ## Build solar-discovery binary
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_DISCOVERY) ./$(CMD_DIR)/$(BINARY_DISCOVERY)

build-renderer: ## Build solar-renderer binary
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_RENDERER) ./$(CMD_DIR)/$(BINARY_RENDERER)

build-agent: ## Build solar-agent binary
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_AGENT) ./$(CMD_DIR)/$(BINARY_AGENT)

##@ Docker

docker-build: ## Build all Docker images
	docker build -t solar-index:$(VERSION) -f Dockerfile.solar-index .
	docker build -t solar-discovery:$(VERSION) -f Dockerfile.solar-discovery .
	docker build -t solar-renderer:$(VERSION) -f Dockerfile.solar-renderer .
	docker build -t solar-agent:$(VERSION) -f Dockerfile.solar-agent .

##@ Dependencies

deps: ## Download dependencies
	$(GOMOD) download

tidy: ## Tidy go modules
	$(GOMOD) tidy

verify: tidy ## Verify dependencies
	$(GOMOD) verify

##@ Tools

tools: $(GOLANGCI_LINT) $(CONTROLLER_GEN) ## Install all tools

$(LOCALBIN):
	mkdir -p $(LOCALBIN)

$(GOLANGCI_LINT): $(LOCALBIN) ## Install golangci-lint
	@test -s $(GOLANGCI_LINT) || \
		GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(CONTROLLER_GEN): $(LOCALBIN) ## Install controller-gen
	@test -s $(CONTROLLER_GEN) || \
		GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

##@ Cleanup

clean: ## Clean build artifacts
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html

.DEFAULT_GOAL := help
