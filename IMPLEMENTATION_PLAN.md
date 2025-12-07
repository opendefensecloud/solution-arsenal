# Solution Arsenal (SolAr) - Detailed Implementation Plan

## Executive Summary

This plan breaks down the SolAr implementation into discrete, testable slices following a backend-first, layer-by-layer approach. Each slice is designed to be independently verifiable with comprehensive tests before moving to the next.

**Current State**: The project has foundational infrastructure in place:
- API Extension Server skeleton with `CatalogItem` resource
- Controller Manager skeleton (empty reconciler)
- Observability package (complete with tests)
- Helm chart infrastructure
- CI/CD pipelines

**Target State**: A fully functional application catalog and fleet management system.

---

## Phase 1: Core API Types (Foundation Layer)

### Slice 1.1: Enhance CatalogItem API

**Goal**: Extend CatalogItem with complete specification and status fields.

**Tasks**:
1. **Internal Type Enhancement** (`api/solar/catalogitem_types.go`)
   - Add `Maintainer` field (name, email)
   - Add `Category` field (enum: application, operator, addon, library)
   - Add `Tags` field (string array for search/filtering)
   - Add `Source` field (struct with registry, path)
   - Add `Attestations` field (array of attestation references)
   - Add `Dependencies` field (array of component dependencies)
   - Add `MinKubernetesVersion` field
   - Add `RequiredCapabilities` field (array of strings)

2. **Status Enhancement**
   - Add `Phase` field (Discovered, Validated, Available, Deprecated)
   - Add `ValidationStatus` (struct with passed checks, failed checks)
   - Add `LastDiscoveredAt` timestamp
   - Add `Conditions` (standard Kubernetes conditions)

3. **Versioned Type** (`api/solar/v1alpha1/catalogitem_types.go`)
   - Mirror internal type changes
   - Add OpenAPI validation markers

4. **Tests**
   - Unit tests for type validation
   - Round-trip conversion tests
   - OpenAPI schema validation

**Verification**: `make test`, `make codegen`, verify OpenAPI specs

---

### Slice 1.2: ClusterCatalogItem API (Cluster-Scoped CatalogItem)

**Goal**: Create cluster-scoped variant of CatalogItem for shared catalog entries.

**Tasks**:
1. Create `api/solar/clustercatalogitem_types.go`
2. Create `api/solar/clustercatalogitem_rest.go` (NamespaceScoped: false)
3. Create `api/solar/v1alpha1/clustercatalogitem_types.go`
4. Register in `register.go` files
5. Add to apiserver `main.go`
6. Comprehensive tests

**Verification**: API accessible at cluster scope, CRUD operations work

---

### Slice 1.3: ClusterRegistration API

**Goal**: Define API for registering target Kubernetes clusters.

**Tasks**:
1. **Types Definition**
   ```
   ClusterRegistrationSpec:
   - DisplayName: string
   - Description: string
   - SecurityDomain: string (from allowed list)
   - Labels: map[string]string
   - RequiredAttestations: []string
   - CapacityInfo: struct (optional, reported by agent)

   ClusterRegistrationStatus:
   - Phase: Pending, Registered, Connected, Disconnected, Error
   - AgentVersion: string
   - LastHeartbeat: timestamp
   - Capacity: struct (CPU, Memory, GPU, Storage)
   - Conditions: []Condition
   - ConnectionInfo: struct (generated on creation)
   ```

2. Create internal and versioned types
3. Create REST strategy with status subresource
4. Register and generate code

**Verification**: Full CRUD, status subresource works independently

---

### Slice 1.4: Release API

**Goal**: Define API for deployment specifications.

**Tasks**:
1. **Types Definition**
   ```
   ReleaseSpec:
   - CatalogItemRef: ObjectReference
   - ClusterRef: ObjectReference (or ClusterSelector)
   - ClusterSelector: LabelSelector (for multi-cluster)
   - Values: runtime.RawExtension (Helm-like values)
   - MaxUsers: int (for scaling calculations)
   - SkipPreflightChecks: bool

   ReleaseStatus:
   - Phase: Pending, Rendering, Deploying, Deployed, Failed, Deleting
   - ClusterStatuses: []ClusterReleaseStatus (per-cluster status)
   - Conditions: []Condition
   - LastRenderedAt: timestamp
   - ManifestDigest: string (OCI digest)
   ```

2. Create internal and versioned types
3. Create REST strategy
4. Register and generate code

**Verification**: Full CRUD, release references validation

---

### Slice 1.5: Sync API

**Goal**: Define API for catalog chaining/syncing between SolAr instances.

**Tasks**:
1. **Types Definition**
   ```
   SyncSpec:
   - CatalogItemSelector: LabelSelector
   - DestinationARC: struct (endpoint, credentials ref)
   - Schedule: string (cron expression, optional)
   - AutoSync: bool

   SyncStatus:
   - Phase: Idle, Syncing, Completed, Failed
   - LastSyncAt: timestamp
   - SyncedItems: []SyncedItemStatus
   - Conditions: []Condition
   ```

2. Create internal and versioned types
3. Create REST strategy
4. Register and generate code

**Verification**: Full CRUD, validation of cron expressions

---

### Slice 1.6: SecurityDomain Configuration API

**Goal**: System-level configuration for allowed security domains.

**Tasks**:
1. **Types Definition**
   ```
   SecurityDomainConfigSpec:
   - AllowedDomains: []string
   - DefaultDomain: string

   SecurityDomainConfigStatus:
   - ActiveDomains: []string
   - ClusterCount: map[string]int (clusters per domain)
   ```

2. Create as cluster-scoped singleton resource
3. Register and generate code

**Verification**: Validation prevents invalid security domains in ClusterRegistration

---

## Phase 2: Controller Business Logic

### Slice 2.1: CatalogItem Controller

**Goal**: Implement reconciliation logic for CatalogItem validation.

**Tasks**:
1. **Controller Logic** (`pkg/controller/catalogitem_controller.go`)
   - Fetch CatalogItem
   - Validate OCM component exists in registry
   - Check attestations if required
   - Update status with validation results
   - Add conditions for each validation step
   - Emit events on state changes

2. **Service Layer** (`internal/index/registry/`)
   - `OCMClient` interface for registry operations
   - `ComponentValidator` for attestation checks
   - Mock implementations for testing

3. **Tests**
   - Unit tests with mocked registry client
   - Integration tests with envtest
   - Table-driven tests for all validation scenarios

**Verification**: `make test`, reconciler correctly updates status

---

### Slice 2.2: ClusterRegistration Controller

**Goal**: Handle cluster registration lifecycle and credential generation.

**Tasks**:
1. **Controller Logic**
   - On create: Generate agent credentials (kubeconfig, tokens)
   - Create agent configuration secret
   - Update status with connection info
   - Monitor heartbeats, update connectivity status
   - Handle finalizer for cleanup

2. **Service Layer**
   - `CredentialGenerator` for agent credentials
   - `HeartbeatMonitor` for connection tracking

3. **Tests**
   - Unit tests for credential generation
   - Integration tests for full lifecycle

**Verification**: ClusterRegistration creates valid agent config

---

### Slice 2.3: Release Controller

**Goal**: Orchestrate release deployment workflow.

**Tasks**:
1. **Controller Logic**
   - Validate catalog item reference exists
   - Validate cluster reference(s) exist
   - Run preflight checks (capacity, capabilities)
   - Trigger renderer (via status update or event)
   - Track per-cluster deployment status
   - Handle rollback scenarios

2. **Service Layer**
   - `PreflightChecker` interface
   - `CapacityCalculator` for resource estimation
   - `CapabilityMatcher` for dependency checking

3. **Tests**
   - Comprehensive preflight check scenarios
   - Multi-cluster deployment tests
   - Failure and rollback tests

**Verification**: Release triggers correct workflow, handles failures

---

### Slice 2.4: Sync Controller

**Goal**: Implement catalog chaining via ARC integration.

**Tasks**:
1. **Controller Logic**
   - Watch Sync resources
   - Select matching CatalogItems
   - Create/update ARC Order resources
   - Track sync status per item
   - Handle scheduled syncs (cron)

2. **Service Layer**
   - `ARCClient` interface for ARC integration
   - Mock ARC client for testing

3. **Tests**
   - Selector matching tests
   - ARC order creation tests
   - Schedule trigger tests

**Verification**: Sync creates correct ARC orders

---

## Phase 3: Discovery Service (solar-discovery)

### Slice 3.1: Discovery Service Skeleton

**Goal**: Create the discovery service binary and basic structure.

**Tasks**:
1. Create `cmd/solar-discovery/main.go`
   - Controller-runtime based
   - Configuration flags for registries
   - Health/readiness probes
   - Metrics endpoint

2. Create `internal/discovery/config/`
   - Configuration types
   - Registry configuration validation

3. **Tests**
   - Main function tests
   - Configuration parsing tests

**Verification**: Binary builds, starts, exposes health endpoints

---

### Slice 3.2: OCI Registry Scanner

**Goal**: Implement registry scanning for OCM packages.

**Tasks**:
1. Create `internal/discovery/controller/scanner.go`
   - Periodic registry scanning
   - Identify OCM components by convention/label
   - Parse component metadata
   - Track discovered vs. existing items

2. **Service Layer**
   - `RegistryScanner` interface
   - `OCMPackageDetector` for identifying valid packages
   - Support for multiple registry types (Docker Registry, Harbor, etc.)

3. **Tests**
   - Mock registry tests
   - Component detection tests
   - Incremental scan tests (only new items)

**Verification**: Scanner discovers test OCM packages correctly

---

### Slice 3.3: CatalogItem Creation

**Goal**: Automatically create CatalogItems from discovered packages.

**Tasks**:
1. Create `internal/discovery/controller/reconciler.go`
   - Convert discovered packages to CatalogItem specs
   - Check for duplicates (same component + version)
   - Create/update CatalogItems via API client
   - Handle cleanup of removed packages (optional)

2. **Tests**
   - End-to-end discovery flow tests
   - Duplicate handling tests
   - Error recovery tests

**Verification**: Discovered packages appear as CatalogItems

---

## Phase 4: Renderer Service (solar-renderer)

### Slice 4.1: Renderer Service Skeleton

**Goal**: Create the renderer service binary.

**Tasks**:
1. Create `cmd/solar-renderer/main.go`
   - Watch Release resources
   - Configuration for OCI push credentials
   - Health/readiness probes

2. Create `internal/renderer/config/`
   - Configuration types
   - OCI registry configuration

3. **Tests**
   - Main function tests
   - Configuration validation tests

**Verification**: Binary builds, connects to apiserver

---

### Slice 4.2: Manifest Rendering Engine

**Goal**: Render OCM components into deployment manifests.

**Tasks**:
1. Create `internal/renderer/engine/`
   - `Renderer` interface
   - OCM component fetching
   - Helm chart processing (if applicable)
   - Value substitution
   - Kustomize overlay support (optional)

2. **Tests**
   - Rendering with various value inputs
   - Template processing tests
   - Error handling tests

**Verification**: Given OCM component, outputs valid K8s manifests

---

### Slice 4.3: OCI Publisher

**Goal**: Push rendered manifests to OCI registry for GitOps consumption.

**Tasks**:
1. Create `internal/renderer/publisher/`
   - Package manifests into OCI image
   - Push to configured registry
   - Generate content digest
   - Update Release status with manifest reference

2. **Tests**
   - OCI push tests (with mock registry)
   - Digest calculation tests
   - Authentication tests

**Verification**: Manifests pushed to OCI, Release status updated

---

### Slice 4.4: Renderer Controller

**Goal**: Orchestrate rendering workflow.

**Tasks**:
1. Create `internal/renderer/controller/`
   - Watch Release resources
   - Queue rendering jobs
   - Handle concurrent rendering
   - Retry logic for transient failures
   - Update Release status throughout process

2. **Tests**
   - Full rendering workflow tests
   - Concurrent release handling
   - Failure recovery tests

**Verification**: Release triggers rendering, manifests appear in registry

---

## Phase 5: Agent Service (solar-agent)

### Slice 5.1: Agent Service Skeleton

**Goal**: Create the agent binary for target clusters.

**Tasks**:
1. Create `cmd/solar-agent/main.go`
   - Bootstrap from agent configuration
   - Connect to solar-index
   - Health/readiness probes
   - Graceful shutdown

2. Create `internal/agent/config/`
   - Agent configuration parsing
   - Credential management
   - Connection validation

3. **Tests**
   - Configuration parsing tests
   - Startup sequence tests

**Verification**: Agent starts, connects to index API

---

### Slice 5.2: Preflight Checks

**Goal**: Validate cluster readiness for deployments.

**Tasks**:
1. Create `internal/agent/preflight/`
   - Kubernetes version check
   - Available capacity check
   - Required CRD presence check
   - Network policy check (optional)
   - Storage class availability check

2. **Tests**
   - Each check type independently
   - Combined preflight report generation

**Verification**: Preflight checks execute, report results

---

### Slice 5.3: FluxCD Integration

**Goal**: Manage FluxCD resources for GitOps deployment.

**Tasks**:
1. Create `internal/agent/flux/`
   - Create/manage OCIRepository resources
   - Create/manage Kustomization resources
   - Watch for reconciliation status
   - Handle Flux errors and retries

2. **Tests**
   - OCIRepository creation tests
   - Kustomization lifecycle tests
   - Status tracking tests

**Verification**: Agent creates Flux resources, deployments occur

---

### Slice 5.4: Status Reporting

**Goal**: Report deployment status back to solar-index.

**Tasks**:
1. Create `internal/agent/status/`
   - Collect deployment status from Flux
   - Collect resource status (pods, services, etc.)
   - Report cluster capacity changes
   - Heartbeat mechanism

2. **Tests**
   - Status collection tests
   - Reporting API tests
   - Heartbeat timing tests

**Verification**: Status appears in Release resources on index

---

### Slice 5.5: Sync Integration

**Goal**: Support catalog chaining from agent side.

**Tasks**:
1. Create `internal/agent/sync/`
   - Watch Sync resources relevant to this cluster
   - Create ARC Order resources locally
   - Report sync status back

2. **Tests**
   - Sync resource watching
   - ARC order creation
   - Status reporting

**Verification**: Sync triggers ARC from target cluster

---

## Phase 6: Authentication & Authorization

### Slice 6.1: RBAC Definitions

**Goal**: Define Kubernetes RBAC for SolAr roles.

**Tasks**:
1. Create ClusterRole definitions for:
   - `solar-solution-maintainer`: CatalogItem CRUD
   - `solar-cluster-maintainer`: ClusterRegistration CRUD
   - `solar-deployment-coordinator`: Release CRUD within scope
   - `solar-admin`: All resources

2. Create RoleBinding templates
3. Document RBAC setup process

**Verification**: Roles work correctly when bound to users

---

### Slice 6.2: Namespace-Based Tenancy

**Goal**: Implement tenant isolation via namespaces.

**Tasks**:
1. Admission webhook for namespace validation
2. Ensure cross-namespace references are blocked
3. Validate ClusterCatalogItem access

**Verification**: Users cannot access resources outside their namespace

---

## Phase 7: Observability Integration

### Slice 7.1: Tracing Integration

**Goal**: Add distributed tracing to all components.

**Tasks**:
1. Add trace context propagation to API handlers
2. Add tracing to controller reconcile loops
3. Add tracing to external calls (registry, ARC)
4. Ensure trace correlation in logs

**Verification**: Traces visible in Jaeger/Zipkin, correlated logs

---

### Slice 7.2: Metrics Instrumentation

**Goal**: Expose Prometheus metrics.

**Tasks**:
1. Controller metrics (reconcile duration, errors)
2. Discovery metrics (scan duration, items found)
3. Renderer metrics (render duration, push duration)
4. Agent metrics (status report latency, flux sync status)
5. ServiceMonitor resources in Helm chart

**Verification**: Metrics scraped by Prometheus, Grafana dashboards

---

### Slice 7.3: Structured Logging

**Goal**: Consistent structured logging across components.

**Tasks**:
1. Apply observability.NewLogger to all components
2. Ensure request IDs flow through
3. Add log level configuration
4. Document log format

**Verification**: Logs are JSON, filterable, correlated

---

## Phase 8: End-to-End Testing

### Slice 8.1: Integration Test Suite

**Goal**: Comprehensive integration tests.

**Tasks**:
1. Set up test fixtures (mock registry, OCM packages)
2. Test discovery -> catalog flow
3. Test registration -> agent flow
4. Test release -> deployment flow
5. Test sync -> ARC flow

**Verification**: `make test-e2e` passes with all flows

---

### Slice 8.2: Performance Tests

**Goal**: Validate system under load.

**Tasks**:
1. Load test with 100+ CatalogItems
2. Load test with 50+ ClusterRegistrations
3. Load test with concurrent Releases
4. Identify and document limits

**Verification**: Performance within acceptable bounds

---

## Phase 9: Helm Chart Completion

### Slice 9.1: New Component Deployments

**Goal**: Add Helm templates for all components.

**Tasks**:
1. Add solar-discovery Deployment
2. Add solar-renderer Deployment
3. Add solar-agent Deployment (separate chart)
4. Add ConfigMap for security domains
5. Add RBAC resources for roles

**Verification**: Full deployment via Helm works

---

### Slice 9.2: Configuration Documentation

**Goal**: Document all configuration options.

**Tasks**:
1. values.yaml documentation
2. Example configurations for common scenarios
3. Troubleshooting guide

**Verification**: Documentation covers all options

---

## Phase 10: Web UI (Next.js Frontend)

### Slice 10.1: UI Scaffolding

**Goal**: Set up Next.js project structure.

**Tasks**:
1. Create `ui/` directory with Next.js app
2. Configure Tailwind CSS
3. Set up API routes proxying to solar-index
4. Authentication integration (OIDC)

**Verification**: UI builds, authenticates users

---

### Slice 10.2: Catalog Explorer

**Goal**: UI for browsing catalog items.

**Tasks**:
1. CatalogItem list view with filtering
2. CatalogItem detail view
3. Search functionality
4. Category/tag navigation

**Verification**: Users can browse and search catalog

---

### Slice 10.3: Cluster Management

**Goal**: UI for cluster registration.

**Tasks**:
1. ClusterRegistration list view
2. Cluster registration wizard
3. Agent config download
4. Cluster status dashboard

**Verification**: Users can register clusters via UI

---

### Slice 10.4: Release Management

**Goal**: UI for creating and managing releases.

**Tasks**:
1. Release creation wizard
2. Cluster selection (single/multi)
3. Values editor (YAML/form)
4. Release status monitoring
5. Rollback functionality

**Verification**: Users can deploy and monitor releases

---

### Slice 10.5: Sync Management

**Goal**: UI for catalog chaining.

**Tasks**:
1. Sync configuration UI
2. Sync status monitoring
3. Manual sync trigger

**Verification**: Users can configure and monitor syncs

---

## Testing Strategy Summary

| Phase | Unit Tests | Integration Tests | E2E Tests |
|-------|-----------|-------------------|-----------|
| 1. API Types | Type validation, conversion | envtest CRUD | - |
| 2. Controllers | Mock services | envtest reconciliation | - |
| 3. Discovery | Mock registry | Discovery flow | Kind + registry |
| 4. Renderer | Mock OCM | Render workflow | Kind + registry |
| 5. Agent | Mock clients | Agent workflow | Kind cluster |
| 6. Auth | - | RBAC tests | Full auth flow |
| 7. Observability | Already done | Trace/metric collection | Dashboard validation |
| 8. E2E | - | - | Full system tests |
| 9. Helm | Template tests | Install tests | Production-like |
| 10. UI | Component tests | API integration | Cypress/Playwright |

---

## Quality Gates

Before completing each slice:

1. **Code Coverage**: >85% for new code
2. **Linting**: `make lint` passes
3. **Tests**: `make test` passes
4. **Documentation**: Code is documented with GoDoc
5. **API Stability**: Breaking changes documented

---

## Dependency Order

```
Phase 1 (API Types)
    ├── Slice 1.1 (CatalogItem)
    ├── Slice 1.2 (ClusterCatalogItem)
    ├── Slice 1.3 (ClusterRegistration)
    ├── Slice 1.4 (Release) ← depends on 1.1, 1.3
    ├── Slice 1.5 (Sync) ← depends on 1.1
    └── Slice 1.6 (SecurityDomain) ← depends on 1.3

Phase 2 (Controllers) ← depends on Phase 1
    ├── Slice 2.1 (CatalogItem Controller)
    ├── Slice 2.2 (ClusterRegistration Controller)
    ├── Slice 2.3 (Release Controller) ← depends on 2.1, 2.2
    └── Slice 2.4 (Sync Controller) ← depends on 2.1

Phase 3 (Discovery) ← depends on 2.1
    ├── Slice 3.1 (Skeleton)
    ├── Slice 3.2 (Scanner)
    └── Slice 3.3 (Creator)

Phase 4 (Renderer) ← depends on 2.3
    ├── Slice 4.1 (Skeleton)
    ├── Slice 4.2 (Engine)
    ├── Slice 4.3 (Publisher)
    └── Slice 4.4 (Controller)

Phase 5 (Agent) ← depends on 2.2, 4.3
    ├── Slice 5.1 (Skeleton)
    ├── Slice 5.2 (Preflight)
    ├── Slice 5.3 (Flux)
    ├── Slice 5.4 (Status)
    └── Slice 5.5 (Sync)

Phase 6 (Auth) ← can start after Phase 1
Phase 7 (Observability) ← can start after Phase 2
Phase 8 (E2E) ← depends on all previous phases
Phase 9 (Helm) ← depends on all components
Phase 10 (UI) ← depends on Phase 1-5
```

---

## Getting Started

To begin implementation, start with **Slice 1.1: Enhance CatalogItem API**. This provides the foundation for all subsequent work and can be verified independently.

```bash
# Verify current state
make test
make lint

# After implementing Slice 1.1
make codegen
make test
make lint
```

Each slice should take the following approach:
1. Write tests first (TDD where practical)
2. Implement minimal code to pass tests
3. Refactor for clarity
4. Verify quality gates
5. Commit with clear message
