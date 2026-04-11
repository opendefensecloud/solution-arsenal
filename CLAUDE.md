# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Solution Arsenal (SolAr) is a Kubernetes application catalog and fleet rollout manager built on Open Component Model (OCM) packages. The backend is implemented as a Kubernetes API Extension Server using [apiserver-kit](https://github.com/opendefensecloud/apiserver-kit). Go module: `go.opendefense.cloud/solar`.

## Build & Development Commands

```bash
make fmt          # Format code, add license headers, run golangci-lint --fix
make lint         # Run all linters (golangci-lint + license checks + shellcheck)
make test         # Run all unit/integration tests (uses ginkgo + envtest)
make test-e2e     # Run e2e tests (requires Kind cluster, see make e2e-cluster)
make codegen      # Run code generation (openapi-gen, controller-gen, CRD ref docs)
make manifests    # Generate RBAC and CRD manifests
make mod          # go mod tidy + download + verify
make scan         # Run osv-scanner for vulnerability scanning
```

### Running a single test

Tests use Ginkgo. Run a specific package:
```bash
# After setup-envtest and ginkgo are installed (make setup-envtest ginkgo)
KUBEBUILDER_ASSETS="$(bin/setup-envtest use 1.34.1 --bin-dir bin -p path)" \
  bin/ginkgo -r -cover --fail-fast ./pkg/renderer/
```

Use `-focus "description"` to run a specific test case within a package.

### Docker & Local Cluster

```bash
make docker-build           # Build all component images
make dev-cluster             # Create Kind cluster with all components
make dev-cluster-rebuild     # Rebuild images and upgrade Helm release in dev cluster
make cleanup-dev-cluster     # Tear down dev Kind cluster
```

### Dev Environment

The project uses [devenv](https://devenv.sh/) (see `devenv.nix`) to provide Go 1.26.1 and system tools (kind, kubectl, helm, fluxcd, shellcheck, yq, jq). Pre-commit hooks run gofmt, golangci-lint, and osv-scanner.

## Architecture

### Components (4 binaries in `cmd/`)

- **solar-apiserver** — Kubernetes extension API server. Serves the SolAr API types as native Kubernetes resources. Uses `go.opendefense.cloud/kit` (apiserver-kit) as its foundation. REST handlers are defined directly on API types in `api/solar/` via `*_rest.go` files.
- **solar-controller-manager** — Runs controllers that reconcile SolAr resources (release, target, rendertask, bootstrap). Controllers live in `pkg/controller/`.
- **solar-renderer** — Watches for desired state changes and renders OCI images containing deployment manifests for FluxCD gitless GitOps. Logic in `pkg/renderer/`.
- **solar-discovery** — Standalone tool that scans OCI registries for OCM packages and updates the catalog. Deployed separately via its own Helm chart (`charts/solar-discovery/`). Pipeline/scanner/handler logic in `pkg/discovery/`.

### Key Directories

- `api/solar/` — API types with REST storage implementations (internal version). REST handlers (`*_rest.go`) are co-located with type definitions (`*_types.go`).
- `api/solar/v1alpha1/` — External (versioned) API types with conversion and defaults. Files prefixed `zz_generated.*` are auto-generated.
- `client-go/` — Generated typed client, informers, listers, and apply configurations (do not edit manually).
- `pkg/controller/` — Controller-runtime reconcilers for each resource type.
- `pkg/discovery/` — Registry scanning pipeline: scanner → qualifier → handler → apiwriter.
- `pkg/renderer/` — Helm chart rendering and OCI push logic for releases and hydrated targets.
- `charts/solar/` — Helm chart for deploying the SolAr API server, controller-manager, and renderer.
- `charts/solar-discovery/` — Helm chart for deploying solar-discovery standalone.
- `hack/` — Code generation and dev cluster setup scripts.

### API Resource Types

The core Kubernetes resources managed by SolAr: `Component`, `ComponentVersion`, `Target`, `Bootstrap`, `Release`, `Profile`, `RenderTask`.

### Code Generation

After modifying API types, run `make codegen` which invokes `hack/update-codegen.sh` to regenerate OpenAPI specs, deepcopy functions, conversions, and client-go code.

## Code Style

- Import order enforced by gci: standard → third-party → local module → blank → dot
- All `.go` files must have Apache license headers (enforced by `addlicense`)
- golangci-lint v2 with extensive linter set (see `.golangci.yaml`)
- Tests use Ginkgo/Gomega BDD framework with envtest for integration tests
- `nlreturn` linter requires blank lines before return statements in blocks ≥2 lines
