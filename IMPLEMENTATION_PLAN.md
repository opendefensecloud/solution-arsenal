# Solution Arsenal (SolAr) Implementation Plan

This document outlines a detailed, phased implementation plan for Solution Arsenal, broken into small, testable slices. Each phase builds upon the previous one, enabling continuous testing and validation.

---

## Table of Contents

1. [Phase 1: Project Foundation](#phase-1-project-foundation)
2. [Phase 2: Domain Models & API Types](#phase-2-domain-models--api-types)
3. [Phase 3: solar-index Core (Extension APIServer)](#phase-3-solar-index-core-extension-apiserver)
4. [Phase 4: solar-index Storage & Persistence](#phase-4-solar-index-storage--persistence)
5. [Phase 5: solar-index API Validation & Admission](#phase-5-solar-index-api-validation--admission)
6. [Phase 6: solar-discovery Component](#phase-6-solar-discovery-component)
7. [Phase 7: solar-renderer Component](#phase-7-solar-renderer-component)
8. [Phase 8: solar-agent Component](#phase-8-solar-agent-component)
9. [Phase 9: solar-ui Frontend](#phase-9-solar-ui-frontend)
10. [Phase 10: Integration, E2E Testing & Deployment](#phase-10-integration-e2e-testing--deployment)

---

## Phase 1: Project Foundation

**Goal**: Establish the Go module structure, shared packages, tooling, and CI pipeline.

### Slice 1.1: Go Module Initialization

**Files to create:**
```
go.mod
go.work (for multi-module workspace)
Makefile
.golangci.yml
.gitignore
```

**Tasks:**
- [ ] Initialize Go module: `module github.com/siemens/solution-arsenal`
- [ ] Set Go version to 1.25+
- [ ] Add core dependencies:
  - `k8s.io/apiserver` (extension apiserver framework)
  - `k8s.io/apimachinery` (API machinery)
  - `k8s.io/client-go` (Kubernetes client)
  - `sigs.k8s.io/controller-runtime` (controller patterns)
  - `github.com/open-component-model/ocm` (OCM SDK)
  - `go.opentelemetry.io/otel` (observability)
- [ ] Create Makefile with targets: `build`, `test`, `lint`, `generate`, `fmt`
- [ ] Configure golangci-lint with strict rules

**Test**: `go mod tidy && make lint` passes

---

### Slice 1.2: Project Directory Structure

**Directories to create:**
```
cmd/
  solar-index/
  solar-discovery/
  solar-renderer/
  solar-agent/
internal/
  index/          # solar-index internal logic
  discovery/      # solar-discovery internal logic
  renderer/       # solar-renderer internal logic
  agent/          # solar-agent internal logic
pkg/
  apis/           # API type definitions (shared)
  client/         # Generated clientsets
  observability/  # Tracing, metrics, logging utilities
  config/         # Configuration loading
  registry/       # OCI registry client utilities
  ocm/            # OCM helpers
api/
  openapi/        # OpenAPI specs
configs/
  samples/        # Sample configuration files
test/
  e2e/            # End-to-end tests
  integration/    # Integration tests
  mocks/          # Shared mocks
hack/
  boilerplate.go.txt  # License header
  update-codegen.sh   # Code generation script
```

**Tasks:**
- [ ] Create directory structure with placeholder `.gitkeep` files
- [ ] Add license boilerplate file
- [ ] Create basic README in each cmd/ subdirectory

**Test**: Directory structure exists, `tree` shows expected layout

---

### Slice 1.3: Shared Observability Package

**Files to create:**
```
pkg/observability/
  tracing.go      # OpenTelemetry tracer setup
  metrics.go      # OpenTelemetry meter setup
  logging.go      # Structured logging with trace correlation
  middleware.go   # HTTP/gRPC instrumentation middleware
  observability_test.go
```

**Tasks:**
- [ ] Implement `InitTracer(ctx, serviceName, endpoint) (*sdktrace.TracerProvider, error)`
- [ ] Implement `InitMeter(ctx, serviceName, endpoint) (*sdkmetric.MeterProvider, error)`
- [ ] Implement structured logger with trace ID injection
- [ ] Create HTTP middleware for automatic span creation
- [ ] Write unit tests with mock exporters

**Test**: `go test ./pkg/observability/...` passes

---

### Slice 1.4: Configuration Package

**Files to create:**
```
pkg/config/
  config.go       # Configuration structs and loading
  validation.go   # Config validation
  config_test.go
```

**Tasks:**
- [ ] Define base configuration struct with common fields (logging level, telemetry endpoint, etc.)
- [ ] Implement configuration loading from file, env vars, and flags
- [ ] Add validation for required fields
- [ ] Write unit tests

**Test**: `go test ./pkg/config/...` passes

---

### Slice 1.5: CI Pipeline Setup

**Files to create:**
```
.github/
  workflows/
    ci.yaml         # Main CI workflow
    release.yaml    # Release workflow
Dockerfile.solar-index
Dockerfile.solar-discovery
Dockerfile.solar-renderer
Dockerfile.solar-agent
```

**Tasks:**
- [ ] Create GitHub Actions workflow for:
  - Go build
  - Lint (golangci-lint)
  - Unit tests with coverage
  - Integration tests (on PR)
- [ ] Create multi-stage Dockerfiles for each component
- [ ] Add dependabot configuration

**Test**: CI workflow runs successfully on push

---

## Phase 2: Domain Models & API Types

**Goal**: Define Kubernetes custom resource types for the entire system.

### Slice 2.1: API Group Registration

**Files to create:**
```
pkg/apis/
  solar/
    register.go           # Group registration
    doc.go
    v1alpha1/
      doc.go
      register.go         # Version registration
      types.go            # Type definitions (empty initially)
      zz_generated.deepcopy.go  # Generated
```

**Tasks:**
- [ ] Define API group: `solar.siemens.com`
- [ ] Define version: `v1alpha1`
- [ ] Set up code generation markers
- [ ] Create `hack/update-codegen.sh` using `k8s.io/code-generator`

**Test**: `make generate` produces deepcopy functions

---

### Slice 2.2: CatalogItem Type

**File**: `pkg/apis/solar/v1alpha1/catalogitem_types.go`

```go
// CatalogItem represents an OCM package available in the catalog
type CatalogItem struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   CatalogItemSpec   `json:"spec,omitempty"`
    Status CatalogItemStatus `json:"status,omitempty"`
}

type CatalogItemSpec struct {
    // ComponentName is the OCM component name
    ComponentName string `json:"componentName"`
    // Version is the semantic version of the component
    Version string `json:"version"`
    // Repository is the OCI repository URL
    Repository string `json:"repository"`
    // Description of the catalog item
    Description string `json:"description,omitempty"`
    // Labels for categorization
    Labels map[string]string `json:"labels,omitempty"`
    // Dependencies lists other catalog items this depends on
    Dependencies []ComponentReference `json:"dependencies,omitempty"`
}

type CatalogItemStatus struct {
    // Phase indicates the current state
    Phase CatalogItemPhase `json:"phase,omitempty"`
    // Conditions for detailed status
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    // LastScanned timestamp of last discovery scan
    LastScanned *metav1.Time `json:"lastScanned,omitempty"`
}
```

**Tasks:**
- [ ] Define CatalogItem struct with spec/status pattern
- [ ] Define CatalogItemSpec with OCM component reference fields
- [ ] Define CatalogItemStatus with phase and conditions
- [ ] Add JSON tags and validation markers
- [ ] Run code generation

**Test**: Types compile, deepcopy generated, JSON marshaling works

---

### Slice 2.3: ClusterCatalogItem Type

**File**: `pkg/apis/solar/v1alpha1/clustercatalogitem_types.go`

**Tasks:**
- [ ] Define ClusterCatalogItem (cluster-scoped variant of CatalogItem)
- [ ] Same spec/status as CatalogItem but without namespace
- [ ] Add validation that only admins can create cluster-scoped items

**Test**: Types compile, deepcopy generated

---

### Slice 2.4: ClusterRegistration Type

**File**: `pkg/apis/solar/v1alpha1/clusterregistration_types.go`

```go
type ClusterRegistration struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
    Status ClusterRegistrationStatus `json:"status,omitempty"`
}

type ClusterRegistrationSpec struct {
    // DisplayName for the cluster
    DisplayName string `json:"displayName"`
    // Description of the cluster purpose
    Description string `json:"description,omitempty"`
    // Labels for cluster categorization
    Labels map[string]string `json:"labels,omitempty"`
    // AgentConfig holds configuration for the solar-agent
    AgentConfig AgentConfiguration `json:"agentConfig,omitempty"`
}

type ClusterRegistrationStatus struct {
    Phase               ClusterPhase       `json:"phase,omitempty"`
    Conditions          []metav1.Condition `json:"conditions,omitempty"`
    AgentVersion        string             `json:"agentVersion,omitempty"`
    KubernetesVersion   string             `json:"kubernetesVersion,omitempty"`
    LastHeartbeat       *metav1.Time       `json:"lastHeartbeat,omitempty"`
    InstalledReleases   []ReleaseReference `json:"installedReleases,omitempty"`
}
```

**Tasks:**
- [ ] Define ClusterRegistration struct
- [ ] Include agent configuration in spec
- [ ] Include cluster health and status fields
- [ ] Run code generation

**Test**: Types compile, deepcopy generated

---

### Slice 2.5: Release Type

**File**: `pkg/apis/solar/v1alpha1/release_types.go`

```go
type Release struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   ReleaseSpec   `json:"spec,omitempty"`
    Status ReleaseStatus `json:"status,omitempty"`
}

type ReleaseSpec struct {
    // CatalogItemRef references the catalog item to deploy
    CatalogItemRef ObjectReference `json:"catalogItemRef"`
    // TargetClusterRef references the target cluster
    TargetClusterRef ObjectReference `json:"targetClusterRef"`
    // Values are the configuration values for the release
    Values runtime.RawExtension `json:"values,omitempty"`
    // Suspend prevents reconciliation if true
    Suspend bool `json:"suspend,omitempty"`
}

type ReleaseStatus struct {
    Phase            ReleasePhase       `json:"phase,omitempty"`
    Conditions       []metav1.Condition `json:"conditions,omitempty"`
    AppliedVersion   string             `json:"appliedVersion,omitempty"`
    LastAppliedTime  *metav1.Time       `json:"lastAppliedTime,omitempty"`
    ObservedGeneration int64            `json:"observedGeneration,omitempty"`
}
```

**Tasks:**
- [ ] Define Release struct linking catalog items to clusters
- [ ] Include values override capability
- [ ] Include suspend mechanism
- [ ] Run code generation

**Test**: Types compile, deepcopy generated

---

### Slice 2.6: Sync Type (for Catalog Chaining)

**File**: `pkg/apis/solar/v1alpha1/sync_types.go`

**Tasks:**
- [ ] Define Sync struct for catalog chaining feature
- [ ] Include source/destination registry references
- [ ] Include filter rules for selective sync
- [ ] Run code generation

**Test**: Types compile, deepcopy generated

---

### Slice 2.7: Generate Clientsets

**Tasks:**
- [ ] Generate typed clientset using code-generator
- [ ] Generate listers and informers
- [ ] Create convenience functions for common operations
- [ ] Write integration tests against envtest

**Test**: `go test ./pkg/client/...` passes

---

## Phase 3: solar-index Core (Extension APIServer)

**Goal**: Implement the Kubernetes extension apiserver skeleton.

### Slice 3.1: APIServer Bootstrap

**Files to create:**
```
cmd/solar-index/
  main.go
  app/
    server.go       # Server configuration
    options.go      # Command-line options
internal/index/
  apiserver/
    apiserver.go    # Extension apiserver setup
```

**Tasks:**
- [ ] Create main.go with cobra command structure
- [ ] Implement server options (etcd, secure serving, auth, etc.)
- [ ] Set up extension apiserver using `k8s.io/apiserver` libraries
- [ ] Configure API group installation
- [ ] Add health and readiness endpoints

**Test**: Server starts and responds to `/healthz`

---

### Slice 3.2: REST Storage Interface

**Files to create:**
```
internal/index/
  registry/
    catalogitem/
      strategy.go     # Create/Update strategy
      storage.go      # REST storage implementation
    clusterregistration/
      strategy.go
      storage.go
    release/
      strategy.go
      storage.go
```

**Tasks:**
- [ ] Implement `rest.Storage` interface for CatalogItem
- [ ] Implement create/update/delete strategies
- [ ] Add field selectors and label selectors
- [ ] Implement table conversion for `kubectl get`

**Test**: CRUD operations work via API

---

### Slice 3.3: Subresources (Status)

**Tasks:**
- [ ] Implement `/status` subresource for CatalogItem
- [ ] Implement `/status` subresource for ClusterRegistration
- [ ] Implement `/status` subresource for Release
- [ ] Ensure status and spec updates are separate

**Test**: Status updates don't modify spec and vice versa

---

### Slice 3.4: Authentication & Authorization Hooks

**Files to create:**
```
internal/index/
  auth/
    authenticator.go  # Token authentication
    authorizer.go     # RBAC integration
```

**Tasks:**
- [ ] Integrate with Kubernetes authentication (ServiceAccount tokens, OIDC)
- [ ] Implement delegated authorization to kube-apiserver
- [ ] Add audit logging hooks
- [ ] Test with different user contexts

**Test**: Unauthorized requests are rejected, authorized succeed

---

### Slice 3.5: OpenAPI Schema Generation

**Tasks:**
- [ ] Configure OpenAPI v3 schema generation
- [ ] Add validation rules via OpenAPI
- [ ] Generate and serve OpenAPI spec at `/openapi/v3`
- [ ] Validate schema with openapi-generator

**Test**: OpenAPI spec is valid, can generate clients

---

## Phase 4: solar-index Storage & Persistence

**Goal**: Implement persistent storage backend.

### Slice 4.1: etcd Storage Backend

**Files to create:**
```
internal/index/
  storage/
    etcd/
      factory.go      # Storage factory
      options.go      # etcd options
```

**Tasks:**
- [ ] Configure etcd3 storage backend
- [ ] Implement storage factory for all resource types
- [ ] Add connection pooling and retry logic
- [ ] Configure TLS for etcd communication

**Test**: Data persists across restarts

---

### Slice 4.2: Watch Support

**Tasks:**
- [ ] Implement watch for all resource types
- [ ] Add bookmark support for efficient watches
- [ ] Test watch reconnection scenarios
- [ ] Add watch cache configuration

**Test**: Watches receive create/update/delete events

---

### Slice 4.3: Storage Metrics

**Tasks:**
- [ ] Add etcd latency metrics
- [ ] Add storage operation counters
- [ ] Add watch metrics
- [ ] Export via OpenTelemetry

**Test**: Metrics appear in `/metrics` endpoint

---

## Phase 5: solar-index API Validation & Admission

**Goal**: Implement validation and mutation webhooks.

### Slice 5.1: Validation Webhook Framework

**Files to create:**
```
internal/index/
  admission/
    validator.go      # Validation webhook handler
    mutator.go        # Mutation webhook handler
    webhook.go        # Webhook server setup
```

**Tasks:**
- [ ] Set up admission webhook server
- [ ] Implement validation interface
- [ ] Add TLS certificate handling
- [ ] Configure webhook registration

**Test**: Invalid resources are rejected

---

### Slice 5.2: CatalogItem Validation

**Tasks:**
- [ ] Validate OCM component reference format
- [ ] Validate semantic version format
- [ ] Validate repository URL format
- [ ] Check for duplicate entries

**Test**: Invalid CatalogItems are rejected with clear errors

---

### Slice 5.3: Release Validation

**Tasks:**
- [ ] Validate catalog item reference exists
- [ ] Validate cluster registration reference exists
- [ ] Validate values schema against catalog item
- [ ] Prevent conflicting releases

**Test**: Invalid Releases are rejected

---

### Slice 5.4: ClusterRegistration Mutation

**Tasks:**
- [ ] Generate unique agent credentials on create
- [ ] Set default agent configuration
- [ ] Add finalizers for cleanup

**Test**: Created ClusterRegistrations have generated credentials

---

## Phase 6: solar-discovery Component

**Goal**: Implement OCI registry scanner for OCM packages.

### Slice 6.1: OCI Registry Client

**Files to create:**
```
pkg/registry/
  client.go         # OCI registry client interface
  oci/
    client.go       # OCI implementation
    auth.go         # Registry authentication
  client_test.go
```

**Tasks:**
- [ ] Define registry client interface
- [ ] Implement OCI registry client using `oras-go` or similar
- [ ] Add authentication support (basic, token, keychain)
- [ ] Add retry and timeout handling
- [ ] Write unit tests with mock registry

**Test**: Can list repositories and tags from test registry

---

### Slice 6.2: OCM Package Scanner

**Files to create:**
```
pkg/ocm/
  scanner.go        # OCM component scanner
  parser.go         # Component descriptor parser
  scanner_test.go
```

**Tasks:**
- [ ] Implement OCM component descriptor parser
- [ ] Detect solar-compatible packages (via labels/annotations)
- [ ] Extract metadata for catalog items
- [ ] Handle version discovery

**Test**: Can parse OCM component descriptors

---

### Slice 6.3: Discovery Controller

**Files to create:**
```
cmd/solar-discovery/
  main.go
internal/discovery/
  controller/
    controller.go     # Main discovery controller
    reconciler.go     # Reconciliation logic
  config/
    config.go         # Discovery configuration
```

**Tasks:**
- [ ] Implement controller-runtime based controller
- [ ] Configure registry scan intervals
- [ ] Implement reconciliation loop
- [ ] Add leader election for HA

**Test**: Controller starts and logs scan activity

---

### Slice 6.4: CatalogItem Synchronization

**Tasks:**
- [ ] Create CatalogItems for discovered components
- [ ] Update existing CatalogItems on changes
- [ ] Handle deleted components (mark as unavailable)
- [ ] Respect namespace boundaries

**Test**: CatalogItems appear after registry scan

---

### Slice 6.5: Discovery Observability

**Tasks:**
- [ ] Add metrics: scan duration, items discovered, errors
- [ ] Add tracing spans for registry operations
- [ ] Implement structured logging with context
- [ ] Add alerting rules for scan failures

**Test**: Metrics and traces visible in observability stack

---

## Phase 7: solar-renderer Component

**Goal**: Implement manifest renderer that watches releases and updates OCI images.

### Slice 7.1: Release Watcher

**Files to create:**
```
cmd/solar-renderer/
  main.go
internal/renderer/
  controller/
    controller.go     # Release watch controller
    reconciler.go     # Reconciliation logic
```

**Tasks:**
- [ ] Implement controller watching Release resources
- [ ] Filter releases by target cluster
- [ ] Handle release create/update/delete events
- [ ] Add leader election

**Test**: Controller logs release events

---

### Slice 7.2: Manifest Rendering Engine

**Files to create:**
```
internal/renderer/
  engine/
    engine.go         # Rendering engine interface
    ocm.go            # OCM-based rendering
    values.go         # Values merging
```

**Tasks:**
- [ ] Define rendering engine interface
- [ ] Implement OCM component localization
- [ ] Implement values merging (defaults + overrides)
- [ ] Handle templating if applicable

**Test**: Engine produces valid manifests from OCM components

---

### Slice 7.3: OCI Image Publisher

**Files to create:**
```
internal/renderer/
  publisher/
    publisher.go      # OCI image publisher
    layering.go       # Image layer management
```

**Tasks:**
- [ ] Implement OCI image creation from manifests
- [ ] Add content-addressable layering
- [ ] Implement atomic publish (push then tag)
- [ ] Handle large manifests efficiently

**Test**: Manifests published to test registry

---

### Slice 7.4: Desired State Management

**Tasks:**
- [ ] Define desired state structure per cluster
- [ ] Implement state diffing
- [ ] Track which releases contribute to state
- [ ] Handle release removal (cleanup)

**Test**: Desired state reflects active releases

---

### Slice 7.5: Renderer Status Updates

**Tasks:**
- [ ] Update Release status after rendering
- [ ] Report rendering errors
- [ ] Track rendered version
- [ ] Add observability

**Test**: Release status reflects render state

---

## Phase 8: solar-agent Component

**Goal**: Implement cluster agent for status reporting and FluxCD integration.

### Slice 8.1: Agent Bootstrap

**Files to create:**
```
cmd/solar-agent/
  main.go
internal/agent/
  config/
    config.go         # Agent configuration
    credentials.go    # Credential management
```

**Tasks:**
- [ ] Create agent entrypoint
- [ ] Load configuration from secret/configmap
- [ ] Establish connection to solar-index
- [ ] Implement credential refresh

**Test**: Agent starts and connects to solar-index

---

### Slice 8.2: Preflight Checks

**Files to create:**
```
internal/agent/
  preflight/
    checks.go         # Preflight check implementations
    runner.go         # Check runner
```

**Tasks:**
- [ ] Check FluxCD installation
- [ ] Check required CRDs
- [ ] Check RBAC permissions
- [ ] Check network connectivity to OCI registry
- [ ] Report check results to solar-index

**Test**: Preflight checks run and report status

---

### Slice 8.3: FluxCD Resource Management

**Files to create:**
```
internal/agent/
  flux/
    ocirepository.go  # OCIRepository management
    kustomization.go  # Kustomization management
    reconciler.go     # FluxCD reconciliation
```

**Tasks:**
- [ ] Create OCIRepository pointing to desired state image
- [ ] Create Kustomization to apply manifests
- [ ] Handle updates when desired state changes
- [ ] Implement cleanup on release removal

**Test**: FluxCD resources created for releases

---

### Slice 8.4: Status Reporter

**Files to create:**
```
internal/agent/
  status/
    collector.go      # Status collection
    reporter.go       # Status reporting to solar-index
```

**Tasks:**
- [ ] Collect FluxCD reconciliation status
- [ ] Collect deployed resource health
- [ ] Report heartbeat to solar-index
- [ ] Handle solar-index unavailability gracefully

**Test**: Status appears in ClusterRegistration

---

### Slice 8.5: Sync Controller (Catalog Chaining)

**Files to create:**
```
internal/agent/
  sync/
    controller.go     # Sync resource controller
    arc.go            # ARC integration
```

**Tasks:**
- [ ] Watch Sync resources in catalog cluster
- [ ] Create ARC Order resources
- [ ] Track sync status
- [ ] Handle sync failures

**Test**: Sync resources trigger ARC orders

---

## Phase 9: solar-ui Frontend

**Goal**: Implement React/Next.js management UI.

### Slice 9.1: Next.js Project Setup

**Files to create:**
```
solar-ui/
  package.json
  tsconfig.json
  tailwind.config.ts
  next.config.ts
  src/
    app/
      layout.tsx
      page.tsx
```

**Tasks:**
- [ ] Initialize Next.js 14+ with App Router
- [ ] Configure TypeScript strict mode
- [ ] Set up TailwindCSS with design tokens
- [ ] Add Shadcn/ui components
- [ ] Configure ESLint and Prettier

**Test**: `npm run dev` starts development server

---

### Slice 9.2: Kubernetes API Client

**Files to create:**
```
solar-ui/src/
  lib/
    kubernetes/
      client.ts       # K8s API client
      types.ts        # TypeScript types for resources
      hooks.ts        # React Query hooks
```

**Tasks:**
- [ ] Implement Kubernetes API client using fetch
- [ ] Generate TypeScript types from OpenAPI
- [ ] Create React Query hooks for CRUD operations
- [ ] Handle authentication (token forwarding)

**Test**: Can fetch CatalogItems from API

---

### Slice 9.3: Authentication Flow

**Files to create:**
```
solar-ui/src/
  app/
    login/
      page.tsx
  lib/
    auth/
      provider.tsx    # Auth context provider
      session.ts      # Session management
```

**Tasks:**
- [ ] Implement OIDC authentication flow
- [ ] Create login page
- [ ] Implement session management
- [ ] Add token refresh

**Test**: User can log in and session persists

---

### Slice 9.4: Catalog Browser

**Files to create:**
```
solar-ui/src/
  app/
    catalog/
      page.tsx              # Catalog list
      [name]/
        page.tsx            # Catalog item detail
  components/
    catalog/
      CatalogList.tsx
      CatalogCard.tsx
      CatalogDetail.tsx
```

**Tasks:**
- [ ] Implement catalog list view with search/filter
- [ ] Implement catalog item detail view
- [ ] Show version history
- [ ] Display dependencies
- [ ] Add accessibility features

**Test**: Can browse and view catalog items

---

### Slice 9.5: Cluster Management

**Files to create:**
```
solar-ui/src/
  app/
    clusters/
      page.tsx              # Cluster list
      new/
        page.tsx            # Register new cluster
      [name]/
        page.tsx            # Cluster detail
  components/
    clusters/
      ClusterList.tsx
      ClusterCard.tsx
      ClusterRegistrationForm.tsx
      AgentConfigDownload.tsx
```

**Tasks:**
- [ ] Implement cluster list view
- [ ] Implement cluster registration form
- [ ] Generate and display agent configuration
- [ ] Show cluster health status
- [ ] List installed releases per cluster

**Test**: Can register cluster and download agent config

---

### Slice 9.6: Release Management

**Files to create:**
```
solar-ui/src/
  app/
    releases/
      page.tsx              # Release list
      new/
        page.tsx            # Create release
      [name]/
        page.tsx            # Release detail
  components/
    releases/
      ReleaseList.tsx
      ReleaseForm.tsx
      ReleaseDetail.tsx
      ValuesEditor.tsx
```

**Tasks:**
- [ ] Implement release list with status indicators
- [ ] Implement release creation wizard
- [ ] Add values editor (YAML/form hybrid)
- [ ] Show release history
- [ ] Add suspend/resume controls

**Test**: Can create and manage releases

---

### Slice 9.7: Dashboard & Overview

**Files to create:**
```
solar-ui/src/
  app/
    dashboard/
      page.tsx
  components/
    dashboard/
      StatsCards.tsx
      RecentActivity.tsx
      ClusterHealthGrid.tsx
```

**Tasks:**
- [ ] Implement dashboard with key metrics
- [ ] Show recent activity feed
- [ ] Display cluster health overview
- [ ] Add quick actions

**Test**: Dashboard displays aggregate data

---

### Slice 9.8: UI Observability

**Tasks:**
- [ ] Add client-side error tracking
- [ ] Implement performance monitoring
- [ ] Add analytics events
- [ ] Configure CSP headers

**Test**: Errors and metrics captured

---

## Phase 10: Integration, E2E Testing & Deployment

**Goal**: Validate the complete system and prepare for production.

### Slice 10.1: Integration Test Infrastructure

**Files to create:**
```
test/
  integration/
    suite_test.go     # Test suite setup
    catalog_test.go
    deployment_test.go
  e2e/
    suite_test.go
    workflow_test.go
```

**Tasks:**
- [ ] Set up envtest for integration tests
- [ ] Create test fixtures for all resource types
- [ ] Implement helper functions for common operations
- [ ] Add CI job for integration tests

**Test**: Integration tests pass in CI

---

### Slice 10.2: End-to-End Test Suite

**Tasks:**
- [ ] Set up kind cluster for E2E
- [ ] Implement full deployment workflow test
- [ ] Test catalog chaining scenario
- [ ] Test failure scenarios
- [ ] Add E2E job to CI

**Test**: E2E tests pass in CI

---

### Slice 10.3: Helm Charts

**Files to create:**
```
charts/
  solar-index/
    Chart.yaml
    values.yaml
    templates/
  solar-discovery/
  solar-renderer/
  solar-agent/
  solar-ui/
```

**Tasks:**
- [ ] Create Helm chart for each component
- [ ] Parameterize all configuration
- [ ] Add RBAC resources
- [ ] Add NetworkPolicies
- [ ] Test installation in clean cluster

**Test**: `helm install` succeeds

---

### Slice 10.4: Documentation

**Files to create:**
```
docs/
  getting-started.md
  architecture.md
  api-reference.md
  deployment.md
  troubleshooting.md
```

**Tasks:**
- [ ] Write getting started guide
- [ ] Document architecture decisions
- [ ] Generate API reference from OpenAPI
- [ ] Write deployment guide
- [ ] Add troubleshooting guide

**Test**: Documentation renders correctly

---

### Slice 10.5: Production Hardening

**Tasks:**
- [ ] Security audit and fixes
- [ ] Performance testing and optimization
- [ ] Add resource limits to all components
- [ ] Configure PodDisruptionBudgets
- [ ] Set up monitoring dashboards
- [ ] Configure alerting rules

**Test**: System passes security scan, handles load

---

## Implementation Order Summary

**Recommended implementation sequence:**

1. **Foundation** (Phase 1) - 1-2 weeks
2. **Domain Models** (Phase 2) - 1 week
3. **solar-index Core** (Phase 3-5) - 2-3 weeks
4. **solar-discovery** (Phase 6) - 1-2 weeks
5. **solar-renderer** (Phase 7) - 1-2 weeks
6. **solar-agent** (Phase 8) - 1-2 weeks
7. **solar-ui** (Phase 9) - 2-3 weeks
8. **Integration & Deployment** (Phase 10) - 1-2 weeks

**Total estimated effort: 10-15 weeks for MVP**

Each slice is designed to be:
- Independently testable
- Incrementally valuable
- Small enough for focused work
- Clear in scope and deliverables

---

## Appendix: Key Dependencies

| Dependency | Purpose | Version |
|------------|---------|---------|
| k8s.io/apiserver | Extension APIServer | v0.29+ |
| k8s.io/client-go | Kubernetes client | v0.29+ |
| sigs.k8s.io/controller-runtime | Controller patterns | v0.17+ |
| github.com/open-component-model/ocm | OCM SDK | latest |
| go.opentelemetry.io/otel | Observability | v1.24+ |
| github.com/fluxcd/flux2 | FluxCD APIs | v2.2+ |
| oras.land/oras-go | OCI client | v2+ |

## Appendix: Testing Strategy

| Level | Tool | Coverage Target |
|-------|------|-----------------|
| Unit | go test | 80%+ |
| Integration | envtest | Core workflows |
| E2E | kind + real clusters | Happy paths |
| Performance | k6/vegeta | Latency SLOs |
| Security | trivy, gosec | No high/critical |
