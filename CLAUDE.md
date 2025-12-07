# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Solution Arsenal (SolAr) is a Kubernetes-native application catalog and fleet deployment management system built on Open Component Model (OCM) packages. The system enables catalog-based solution management with GitOps-style deployments via FluxCD.

## Build Commands

```bash
# Build all binaries
make build

# Run tests with coverage
go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Run linting
golangci-lint run --timeout=5m

# Generate Kubernetes types (deepcopy, clients)
make generate
# Or directly:
controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/..."

# Build individual components
go build -o bin/solar-index ./cmd/solar-index
go build -o bin/solar-discovery ./cmd/solar-discovery
go build -o bin/solar-renderer ./cmd/solar-renderer
go build -o bin/solar-agent ./cmd/solar-agent
```

## Architecture

SolAr consists of five components:

- **solar-index**: Extension API server providing the Kubernetes API for catalog items, cluster registrations, and releases. Built using `k8s.io/apiserver` libraries (reference: https://github.com/opendefensecloud/apiserver-kit).
- **solar-discovery**: Controller that scans OCI registries for OCM packages and creates CatalogItem resources.
- **solar-renderer**: Watches Release resources and renders deployment manifests as OCI images for Gitless GitOps.
- **solar-agent**: Runs in target clusters, manages FluxCD resources, reports status back to solar-index.
- **solar-ui**: Next.js frontend with TailwindCSS for catalog browsing and deployment management.

### Key Kubernetes Resources

| Resource | Scope | Purpose |
|----------|-------|---------|
| CatalogItem | Namespaced | OCM package available in catalog |
| ClusterCatalogItem | Cluster | Global OCM package |
| ClusterRegistration | Namespaced | Target cluster registration |
| Release | Namespaced | Deployment of catalog item to cluster |
| Sync | Namespaced | Catalog chaining configuration |

## Project Layout

```
cmd/                    # Application entrypoints
internal/               # Component-specific logic (index, discovery, renderer, agent)
pkg/
  apis/solar/v1alpha1/  # Kubernetes API types
  client/               # Generated clientsets
  observability/        # OpenTelemetry tracing/metrics/logging
  config/               # Configuration loading
  registry/             # OCI registry client
  ocm/                  # OCM SDK helpers
charts/                 # Helm charts per component
solar-ui/               # Next.js frontend
test/e2e/               # End-to-end tests
test/integration/       # Integration tests with envtest
```

## Development Guidelines

### Backend (Go)

- Target Go 1.25+ with strict golangci-lint configuration
- Use Clean Architecture: handlers → services → repositories → domain models
- All public functions should interact with interfaces, not concrete types
- Use `fmt.Errorf("context: %w", err)` for error wrapping
- OpenTelemetry for all observability (traces, metrics, structured logs with trace correlation)
- Table-driven tests with parallel execution

### Frontend (Next.js)

- App Router (Next.js 14+) with TypeScript strict mode
- TailwindCSS for all styling (no CSS files)
- Shadcn/ui components
- Event handlers prefixed with "handle" (e.g., `handleClick`)
- Use `const` for function definitions with explicit types
- Include accessibility attributes (tabindex, aria-label, keyboard handlers)

## Key Dependencies

- `k8s.io/apiserver` - Extension API server framework
- `k8s.io/client-go` - Kubernetes client
- `sigs.k8s.io/controller-runtime` - Controller patterns
- `github.com/open-component-model/ocm` - OCM SDK
- `go.opentelemetry.io/otel` - Observability
- `oras.land/oras-go/v2` - OCI client

## External References

- OCM Controllers: https://ocm.software/docs/concepts/ocm-controllers/
- apiserver-kit: https://github.com/opendefensecloud/apiserver-kit
- ARC (for catalog chaining): https://github.com/opendefensecloud/artifact-conduit/
