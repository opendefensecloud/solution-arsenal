# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Solution Arsenal (SolAr) is an application catalog based on Open Component Model (OCM) packages with fleet rollout management for Kubernetes clusters. It provides:
- A catalog of solutions (application bundles as OCM packages from OCI registries)
- Cluster registration and deployment target management
- Deployment via OCM Controllers with FluxCD

## Components

- **solar-index**: Catalog data and cluster registrations (Kubernetes extension apiserver)
- **solar-discovery**: OCI registry scanner for OCM packages
- **solar-ui**: Management UI for catalog exploration and deployments
- **solar-renderer**: Watches solar-index for state updates, renders deployment manifests to OCI images
- **solar-agent**: Runs in registered clusters, updates cluster status

## Architecture

The backend is implemented as a Kubernetes API Extension Server based on the kubernetes sample-apiserver pattern.

## Project Structure (Go Backend)

```
cmd/           # Application entrypoints
internal/      # Core application logic (not exposed externally)
pkg/           # Shared utilities and packages
api/           # gRPC/REST transport definitions and handlers
configs/       # Configuration schemas and loading
test/          # Test utilities, mocks, and integration tests
```

## Development Guidelines

### Backend (Go)

- Apply Clean Architecture: handlers/controllers → services/use cases → repositories → domain models
- Interface-driven development with explicit dependency injection
- Handle errors explicitly with wrapped errors: `fmt.Errorf("context: %w", err)`
- Use context propagation for request-scoped values, deadlines, and cancellations
- Use OpenTelemetry for tracing, metrics, and structured logging
- Table-driven tests with parallel execution
- Enforce formatting with `go fmt`, `goimports`, and `golangci-lint`

### Frontend (React/Next.js/TypeScript)

- Use TailwindCSS for all styling (avoid CSS files)
- Use early returns for readability
- Event handlers prefixed with "handle" (e.g., `handleClick`)
- Use const arrow functions with types: `const toggle = (): void => {}`
- Include accessibility attributes (tabindex, aria-label, keyboard handlers)
