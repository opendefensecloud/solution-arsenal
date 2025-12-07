# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Solution Arsenal (SolAr) is an application catalog and fleet rollout management system for Kubernetes clusters, built on Open Component Model (OCM) packages. It implements a Kubernetes API Extension Server pattern using `go.opendefense.cloud/kit`.

## Build and Development Commands

```bash
# Run all tests (uses Ginkgo + envtest)
make test

# Run a single test file
go test ./path/to/package -run TestName

# Run e2e tests (creates Kind cluster)
make test-e2e

# Lint code
make lint

# Format code and add license headers
make fmt

# Generate code (OpenAPI, deepcopy, client-go)
make codegen

# Generate RBAC manifests
make manifests

# Tidy, download, verify modules
make mod

# Build Docker images
make docker-build

# Local development cluster setup
make dev-cluster           # Create Kind cluster with cert-manager and Solar
make dev-cluster-rebuild   # Rebuild and redeploy images to dev cluster
make cleanup-dev-cluster   # Tear down dev cluster
```

## Architecture

### Components

- **solar-apiserver** (`cmd/solar-apiserver/`): Kubernetes API Extension Server exposing custom resources. Uses `go.opendefense.cloud/kit/apiserver` builder pattern.
- **solar-controller-manager** (`cmd/solar-controller-manager/`): Controller-runtime based controllers reconciling Solar resources.
- **solar-discovery**: Scans OCI registries for OCM packages and creates CatalogItems (planned).
- **solar-renderer**: Renders deployment manifests to OCI images for GitOps (planned).
- **solar-agent**: Runs in target clusters, reports status, manages FluxCD resources (planned).

### API Types Location

- Internal types: `api/solar/` (e.g., `catalogitem_types.go`)
- Versioned types: `api/solar/v1alpha1/`
- Generated client code: `client-go/`

### Adding New API Resources

1. Define internal type in `api/solar/<type>_types.go`
2. Define versioned type in `api/solar/v1alpha1/<type>_types.go`
3. Add REST strategy in `api/solar/<type>_rest.go`
4. Register in `api/solar/register.go` and `api/solar/v1alpha1/register.go`
5. Run `make codegen` to generate deepcopy, conversion, and client code
6. Register resource in apiserver main.go using `apiserver.Resource()`

### Observability Package

`pkg/observability/` provides OpenTelemetry integration:
- `InitTracer()` / `InitMeter()` for OTLP exporters
- `NewLogger()` for structured logging with trace correlation
- `HTTPMiddleware()` for automatic request tracing

### Helm Chart

`charts/solar/` contains deployment manifests for:
- API server with etcd backend
- Controller manager
- cert-manager integration for TLS

## Code Style

- Use Clean Architecture: handlers -> services -> repositories -> domain
- Interface-driven development with dependency injection
- Table-driven tests with Ginkgo/Gomega
- Wrap errors with context: `fmt.Errorf("context: %w", err)`
- Use `context.Context` for request-scoped values and cancellation

## Module Configuration

```bash
export GOPRIVATE=*.go.opendefense.cloud/solar
export GNOSUMDB=*.go.opendefense.cloud/solar
export GNOPROXY=*.go.opendefense.cloud/solar
```
