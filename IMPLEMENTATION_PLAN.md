# Solution Arsenal (SolAr) Implementation Plan

This document provides a comprehensive, phased implementation plan for Solution Arsenal. Each phase is broken into small, independently testable slices with clear acceptance criteria.

**Last Updated**: December 2025

---

## Current Implementation Status

> **Note**: The repository includes boilerplate implementation from `go.opendefense.cloud/kit`. This section summarizes what exists and what the plan adapts accordingly.

### ✅ Already Implemented

| Component | Status | Notes |
|-----------|--------|-------|
| **Go Module** | ✅ Complete | `go.opendefense.cloud/solar`, Go 1.25.2 |
| **Dependencies** | ✅ Complete | k8s.io v0.34.2, controller-runtime v0.22.4, OTel v1.38.0 |
| **API Types (CatalogItem)** | ✅ Complete | Basic spec/status, internal + v1alpha1 versions |
| **Extension APIServer** | ✅ Complete | Uses `go.opendefense.cloud/kit/apiserver` |
| **Controller Manager** | ✅ Skeleton | Empty reconciler, needs business logic |
| **Generated Clients** | ✅ Complete | clientset, informers, listers, OpenAPI |
| **Helm Charts** | ✅ Complete | apiserver, controller, etcd, cert-manager integration |
| **CI/CD Workflows** | ✅ Complete | lint, test, docker, helm-lint, helm-publish |
| **Makefile** | ✅ Complete | codegen, fmt, lint, test, docker-build, dev-cluster |
| **E2E Test Framework** | ✅ Complete | kind cluster setup, basic deployment tests |

### ⚠️ Architectural Differences from Original Plan

The existing implementation differs from the original plan in several ways:

1. **Module Path**: Uses `go.opendefense.cloud/solar` instead of `github.com/opendefensecloud/solution-arsenal`
2. **API Location**: Types in `api/solar/` instead of `pkg/apis/solar/`
3. **APIServer Kit**: Uses `go.opendefense.cloud/kit/apiserver` (simplified builder pattern)
4. **Binary Names**: `solar-apiserver` and `solar-controller-manager` instead of `solar-index`
5. **Single Resource**: Only `CatalogItem` defined; other types (ClusterRegistration, Release, Sync) need to be added

### 🔄 What Needs to be Done

1. **Extend API Types**: Add ClusterCatalogItem, ClusterRegistration, Release, Sync types
2. **Implement Controller Logic**: CatalogItem reconciler is empty
3. **Add solar-discovery**: New component for OCI registry scanning
4. **Add solar-renderer**: New component for manifest rendering
5. **Add solar-agent**: New component for target cluster management
6. **Add solar-ui**: Frontend application
7. **Add Observability Package**: Tracing, metrics, structured logging
8. **Add Config Package**: Configuration loading and validation
9. **Extend Helm Charts**: Add new components
10. **Implement Business Logic**: All reconciliation and processing logic

---

## Table of Contents

1. [Version Matrix](#version-matrix)
2. [Phase 1: Project Foundation](#phase-1-project-foundation)
3. [Phase 2: Domain Models & API Types](#phase-2-domain-models--api-types)
4. [Phase 3: solar-index Core (Extension APIServer)](#phase-3-solar-index-core-extension-apiserver)
5. [Phase 4: solar-index Storage & Persistence](#phase-4-solar-index-storage--persistence)
6. [Phase 5: solar-index Validation & Admission](#phase-5-solar-index-validation--admission)
7. [Phase 6: solar-discovery Component](#phase-6-solar-discovery-component)
8. [Phase 7: solar-renderer Component](#phase-7-solar-renderer-component)
9. [Phase 8: solar-agent Component](#phase-8-solar-agent-component)
10. [Phase 9: solar-ui Frontend](#phase-9-solar-ui-frontend)
11. [Phase 10: Integration, E2E Testing & Deployment](#phase-10-integration-e2e-testing--deployment)
12. [Testing Strategy](#testing-strategy)
13. [Dependency Graph](#dependency-graph)

---

## Version Matrix

All dependencies verified as of December 2025. **Versions marked with ✓ are already in go.mod**.

### Backend (Go)

| Dependency | Version | Status | Purpose |
|------------|---------|--------|---------|
| Go | 1.25.2 | ✓ Installed | Runtime |
| k8s.io/apiserver | v0.34.2 | ✓ Installed | Extension APIServer framework |
| k8s.io/client-go | v0.34.2 | ✓ Installed | Kubernetes client |
| k8s.io/apimachinery | v0.34.2 | ✓ Installed | API machinery |
| sigs.k8s.io/controller-runtime | v0.22.4 | ✓ Installed | Controller patterns |
| sigs.k8s.io/controller-tools | v0.19.0 | ✓ Makefile | Code generation (controller-gen) |
| go.opendefense.cloud/kit | v0.1.2 | ✓ Installed | APIServer builder framework |
| go.opentelemetry.io/otel | v1.38.0 | ✓ Indirect | Tracing, metrics, logging |
| ocm.software/ocm | v0.31.0 | 🔲 To Add | Open Component Model SDK |
| oras.land/oras-go/v2 | v2.6.0 | 🔲 To Add | OCI registry client |
| golangci-lint | v2.5.0 | ✓ Makefile | Linting |

### Frontend (Node.js)

| Dependency | Version | Purpose | Source |
|------------|---------|---------|--------|
| Node.js | 22.x LTS | Runtime | [nodejs.org](https://nodejs.org/) |
| Next.js | 16.x | React framework | [Next.js releases](https://github.com/vercel/next.js/releases) |
| React | 19.2.x | UI library | [React versions](https://react.dev/versions) |
| TailwindCSS | 4.1.x | Styling | [TailwindCSS releases](https://github.com/tailwindlabs/tailwindcss/releases) |
| shadcn/ui | latest | Component library | [shadcn/ui](https://ui.shadcn.com/) |
| TypeScript | 5.7.x | Type safety | [TypeScript releases](https://github.com/microsoft/TypeScript/releases) |

### Infrastructure & Testing

| Tool | Version | Purpose | Source |
|------|---------|---------|--------|
| Kubernetes | 1.34.x | Target platform | [Kubernetes releases](https://kubernetes.io/releases/) |
| Helm | 4.0.x | Package management | [Helm releases](https://github.com/helm/helm/releases) |
| FluxCD | v2.7.x | GitOps deployer | [flux2 releases](https://github.com/fluxcd/flux2/releases) |
| kind | v0.30.0 | Local K8s clusters | [kind releases](https://github.com/kubernetes-sigs/kind/releases) |
| envtest | v0.22.x | Integration testing | Part of controller-runtime |

---

## Phase 1: Project Foundation

**Goal**: Establish Go module structure, shared packages, tooling, and CI pipeline.

**Test Coverage Target**: 70%+ for utility packages

**Status**: ✅ Mostly Complete (see notes below)

### Slice 1.1: Go Module Initialization ✅ COMPLETE

**Files created:**

- `go.mod` - Module: `go.opendefense.cloud/solar` (Go 1.25.2)
- `Makefile` - Comprehensive build targets

**Completed Tasks:**

- [x] Initialize Go module: `module go.opendefense.cloud/solar`
- [x] Set Go version to 1.25.2
- [x] Add core dependencies at versions from matrix
- [x] Create Makefile with targets: `codegen`, `test`, `lint`, `manifests`, `fmt`, `mod`, `docker-build`
- [x] golangci-lint v2.5.0 configured (via Makefile download)

**Note**: `.golangci.yml` config file not yet created - linter uses defaults. Consider adding explicit config.

**Acceptance Criteria:**

- [x] `go mod tidy` completes without errors
- [x] `make lint` passes
- [x] `go build ./...` succeeds

---

### Slice 1.2: Project Directory Structure ✅ PARTIAL

**Existing structure** (differs from original plan):

```
api/                          # API types (not pkg/apis/)
  solar/
    v1alpha1/
    install/
    fuzzer/
charts/                       # Helm charts
  solar/
client-go/                    # Generated clients (not pkg/client/)
  clientset/
  informers/
  listers/
  applyconfigurations/
  openapi/
cmd/
  solar-apiserver/            # Extension API server
  solar-controller-manager/   # Controller manager
pkg/
  controller/                 # Controller implementations
test/
  e2e/
  fixtures/
hack/
  boilerplate.go.txt
  update-codegen.sh
docs/
```

**Completed Tasks:**

- [x] Create base directory structure
- [x] Create license boilerplate file (`hack/boilerplate.go.txt`)
- [x] Add main.go in `cmd/solar-apiserver/` and `cmd/solar-controller-manager/`

**Remaining Tasks:**

- [ ] Add `cmd/solar-discovery/` directory and main.go
- [ ] Add `cmd/solar-renderer/` directory and main.go
- [ ] Add `cmd/solar-agent/` directory and main.go
- [ ] Add `pkg/observability/` package
- [ ] Add `pkg/config/` package
- [ ] Add `pkg/registry/` package (OCI client)
- [ ] Add `pkg/ocm/` package (OCM SDK helpers)
- [ ] Add `solar-ui/` directory for frontend

**Acceptance Criteria:**

- [x] Base directory structure exists
- [x] Boilerplate file contains Apache 2.0 header
- [ ] All component directories created

---

### Slice 1.3: Shared Observability Package

**Files to create:**
```
pkg/observability/
  doc.go
  tracing.go
  metrics.go
  logging.go
  middleware.go
  observability_test.go
```

**Tasks:**
- [ ] Implement `InitTracer(ctx context.Context, cfg TracerConfig) (*sdktrace.TracerProvider, error)`
  - Configure OTLP exporter
  - Set resource attributes (service name, version, environment)
  - Configure sampling strategy
- [ ] Implement `InitMeter(ctx context.Context, cfg MeterConfig) (*sdkmetric.MeterProvider, error)`
  - Configure OTLP exporter
  - Set up runtime metrics collection
- [ ] Implement structured logger with trace ID injection
  - Use `slog` with JSON handler
  - Extract trace/span IDs from context
  - Add standard fields (service, version)
- [ ] Create HTTP middleware for automatic span creation
  - Extract/inject W3C trace context
  - Record request duration, status code, path
- [ ] Create gRPC interceptors for tracing
- [ ] Write unit tests with mock exporters

**API Design:**
```go
type TracerConfig struct {
    ServiceName    string
    ServiceVersion string
    Environment    string
    Endpoint       string // OTLP endpoint
    Insecure       bool
    SamplingRatio  float64
}

type MeterConfig struct {
    ServiceName    string
    ServiceVersion string
    Endpoint       string
    Insecure       bool
    Interval       time.Duration
}

func InitTracer(ctx context.Context, cfg TracerConfig) (*sdktrace.TracerProvider, error)
func InitMeter(ctx context.Context, cfg MeterConfig) (*sdkmetric.MeterProvider, error)
func NewLogger(serviceName string) *slog.Logger
func HTTPMiddleware(next http.Handler) http.Handler
func UnaryServerInterceptor() grpc.UnaryServerInterceptor
```

**Acceptance Criteria:**
- [ ] `go test ./pkg/observability/... -cover` shows 70%+ coverage
- [ ] Tracer initializes and exports to mock endpoint
- [ ] Logger outputs JSON with trace IDs when context contains span
- [ ] Middleware creates spans with correct attributes

---

### Slice 1.4: Configuration Package

**Files to create:**
```
pkg/config/
  doc.go
  config.go
  loader.go
  validation.go
  config_test.go
```

**Tasks:**
- [ ] Define base configuration struct with common fields
- [ ] Implement configuration loading from:
  - YAML/JSON file
  - Environment variables (with prefix `SOLAR_`)
  - Command-line flags
- [ ] Implement configuration validation with detailed errors
- [ ] Support configuration hot-reload for select fields
- [ ] Write comprehensive unit tests

**API Design:**
```go
type BaseConfig struct {
    LogLevel       string        `yaml:"logLevel" env:"SOLAR_LOG_LEVEL" default:"info"`
    LogFormat      string        `yaml:"logFormat" env:"SOLAR_LOG_FORMAT" default:"json"`
    TelemetryEndpoint string     `yaml:"telemetryEndpoint" env:"SOLAR_TELEMETRY_ENDPOINT"`
    MetricsPort    int           `yaml:"metricsPort" env:"SOLAR_METRICS_PORT" default:"8080"`
    HealthPort     int           `yaml:"healthPort" env:"SOLAR_HEALTH_PORT" default:"8081"`
}

func Load[T any](configPath string) (*T, error)
func Validate(cfg any) error
func Watch(configPath string, onChange func()) error
```

**Acceptance Criteria:**
- [ ] `go test ./pkg/config/... -cover` shows 70%+ coverage
- [ ] Config loads from file, env vars override file values
- [ ] Missing required fields return descriptive validation errors
- [ ] Invalid enum values are rejected

---

### Slice 1.5: CI Pipeline Setup ✅ COMPLETE

**Existing files:**

- `.github/workflows/golang.yaml` - lint and test jobs
- `.github/workflows/docker.yaml` - Docker image building
- `.github/workflows/helm-lint.yaml` - Helm chart linting
- `.github/workflows/helm-publish.yaml` - Helm chart publishing
- `.github/workflows/release-drafter.yaml` - Automated release notes
- `.github/dependabot.yml` - Dependency updates
- `Dockerfile` - Multi-target build for apiserver and manager

**Completed Tasks:**

- [x] GitHub Actions workflow for Go 1.25.2
- [x] golangci-lint v2.5.0 in CI
- [x] Test coverage reporting (Coveralls integration)
- [x] Multi-stage Dockerfile using distroless base
- [x] Configure Dependabot for Go modules and GitHub Actions

**Remaining Tasks:**

- [ ] Add Trivy scanning for container images
- [ ] Add coverage threshold enforcement (currently reports but doesn't enforce)
- [ ] Add Dockerfiles for solar-discovery, solar-renderer, solar-agent

**Dockerfile template:**
```dockerfile
# Build stage
FROM golang:1.25-bookworm AS builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG COMPONENT
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app ./cmd/${COMPONENT}

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app /app
USER 65532:65532
ENTRYPOINT ["/app"]
```

**Acceptance Criteria:**
- [ ] CI workflow runs lint, test, build on PR
- [ ] Test coverage is reported and enforced (70% minimum for phase 1)
- [ ] Docker images build successfully for all components
- [ ] Trivy scan reports no critical/high vulnerabilities
- [ ] Generated code verification passes

---

### Slice 1.6: Development Environment

**Files to create:**
```
.devcontainer/
  devcontainer.json
  Dockerfile
scripts/
  setup-dev.sh
  run-local.sh
```

**Tasks:**
- [ ] Create devcontainer configuration for VS Code
- [ ] Include Go 1.25, Node.js 22, kubectl, helm, kind
- [ ] Create setup script for local development
- [ ] Create script to run local kind cluster with dependencies
- [ ] Document development setup in CONTRIBUTING.md

**Acceptance Criteria:**
- [ ] Devcontainer builds and starts successfully
- [ ] `scripts/setup-dev.sh` installs all dependencies
- [ ] `scripts/run-local.sh` creates kind cluster and deploys prerequisites

---

## Phase 2: Domain Models & API Types

**Goal**: Define Kubernetes custom resource types for the entire system.

**Test Coverage Target**: 75%+ for type validation

**Status**: ✅ Partial - CatalogItem complete, other types need to be added

### Slice 2.1: API Group Registration ✅ COMPLETE

**Existing files** (note: in `api/` not `pkg/apis/`):

```text
api/solar/
  doc.go
  register.go
  catalogitem_types.go
  catalogitem_rest.go
  v1alpha1/
    doc.go
    register.go
    catalogitem_types.go
    defaults.go
    zz_generated.*.go
  install/
    install.go
  fuzzer/
    fuzzer.go
hack/
  update-codegen.sh
  boilerplate.go.txt
```

**Completed Tasks:**

- [x] Define API group: `solar.opendefense.cloud`
- [x] Define version: `v1alpha1`
- [x] Set up code generation markers
- [x] Create code generation scripts (`hack/update-codegen.sh`)
- [x] Configure generation for DeepCopy, OpenAPI, conversion, defaults

**Note**: Uses `k8s.io/code-generator` pattern with internal/external versions, not kubebuilder.

**Acceptance Criteria:**

- [x] `make codegen` produces deepcopy, conversion, defaults functions
- [x] Generated OpenAPI schema is valid
- [x] Code generation is reproducible

---

### Slice 2.2: Common Types & Helpers 🔲 TODO

**Files to create** (in `api/solar/` and `api/solar/v1alpha1/`):

```text
api/solar/
  common_types.go
  condition_types.go
  reference_types.go
api/solar/v1alpha1/
  common_types.go
  condition_types.go
  reference_types.go
```

**Tasks:**

- [ ] Define common condition types following Kubernetes conventions
- [ ] Define object reference types (local, cross-namespace, cluster-scoped)
- [ ] Define component reference type for OCM
- [ ] Add helper functions for condition management

**Type definitions:**
```go
// ConditionType represents the type of condition
type ConditionType string

const (
    // ConditionReady indicates the resource is ready
    ConditionReady ConditionType = "Ready"
    // ConditionReconciling indicates the resource is being reconciled
    ConditionReconciling ConditionType = "Reconciling"
    // ConditionStalled indicates the resource reconciliation is stalled
    ConditionStalled ConditionType = "Stalled"
)

// ObjectReference references a Kubernetes object
type ObjectReference struct {
    // Name of the referenced object
    Name string `json:"name"`
    // Namespace of the referenced object (optional for cluster-scoped)
    // +optional
    Namespace string `json:"namespace,omitempty"`
}

// ComponentReference references an OCM component
type ComponentReference struct {
    // Name is the OCM component name
    Name string `json:"name"`
    // Version is the semantic version
    Version string `json:"version"`
    // Repository is the OCI repository URL
    Repository string `json:"repository"`
}

// SetCondition sets or updates a condition
func SetCondition(conditions *[]metav1.Condition, condition metav1.Condition)
// GetCondition returns a condition by type
func GetCondition(conditions []metav1.Condition, conditionType ConditionType) *metav1.Condition
// IsReady returns true if the Ready condition is True
func IsReady(conditions []metav1.Condition) bool
```

**Acceptance Criteria:**
- [ ] Types compile without errors
- [ ] Helper functions have 80%+ test coverage
- [ ] Conditions follow Kubernetes API conventions

---

### Slice 2.3: CatalogItem Type ✅ PARTIAL (needs enhancement)

**Existing file**: `api/solar/v1alpha1/catalogitem_types.go`

**Implemented CatalogItemSpec:**

```go
type CatalogItemSpec struct {
    ComponentName string `json:"componentName"`  // OCM component name
    Version       string `json:"version"`        // Semantic version
    Repository    string `json:"repository"`     // OCI repository URL
    Description   string `json:"description,omitempty"`
}
```

**Completed Tasks:**

- [x] Define CatalogItem struct with spec/status pattern
- [x] Define basic CatalogItemSpec with OCM component reference fields
- [x] Add JSON tags
- [x] Run code generation

**Remaining Tasks (to enhance):**

- [ ] Add CatalogItemStatus with phase, conditions, and metadata
- [ ] Add printer columns for kubectl output
- [ ] Add DisplayName, Category, Labels fields
- [ ] Add Dependencies field
- [ ] Add RequiredAttestations field
- [ ] Add ValuesSchema and DefaultValues fields

**Proposed enhanced type definition:**

**Type definition:**
```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Component",type=string,JSONPath=`.spec.component.name`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.component.version`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// CatalogItem represents an OCM package available in the catalog
type CatalogItem struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   CatalogItemSpec   `json:"spec,omitempty"`
    Status CatalogItemStatus `json:"status,omitempty"`
}

type CatalogItemSpec struct {
    // Component is the OCM component reference
    // +kubebuilder:validation:Required
    Component ComponentReference `json:"component"`

    // DisplayName is a human-readable name
    // +optional
    DisplayName string `json:"displayName,omitempty"`

    // Description of the catalog item
    // +optional
    Description string `json:"description,omitempty"`

    // Category for catalog organization
    // +optional
    Category string `json:"category,omitempty"`

    // Labels for additional categorization
    // +optional
    Labels map[string]string `json:"labels,omitempty"`

    // Dependencies lists other components this depends on
    // +optional
    Dependencies []ComponentReference `json:"dependencies,omitempty"`

    // RequiredAttestations lists attestation types required before deployment
    // +optional
    RequiredAttestations []string `json:"requiredAttestations,omitempty"`

    // ValuesSchema is a JSON Schema for configuration values
    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    ValuesSchema *runtime.RawExtension `json:"valuesSchema,omitempty"`

    // DefaultValues are default configuration values
    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    DefaultValues *runtime.RawExtension `json:"defaultValues,omitempty"`
}

// CatalogItemPhase represents the lifecycle phase
// +kubebuilder:validation:Enum=Pending;Available;Deprecated;Archived
type CatalogItemPhase string

const (
    CatalogItemPhasePending    CatalogItemPhase = "Pending"
    CatalogItemPhaseAvailable  CatalogItemPhase = "Available"
    CatalogItemPhaseDeprecated CatalogItemPhase = "Deprecated"
    CatalogItemPhaseArchived   CatalogItemPhase = "Archived"
)

type CatalogItemStatus struct {
    // Phase indicates the current lifecycle state
    // +optional
    Phase CatalogItemPhase `json:"phase,omitempty"`

    // Conditions represent the latest available observations
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // LastScanned is the timestamp of the last discovery scan
    // +optional
    LastScanned *metav1.Time `json:"lastScanned,omitempty"`

    // AvailableVersions lists all discovered versions
    // +optional
    AvailableVersions []string `json:"availableVersions,omitempty"`

    // Attestations lists verified attestations for this component
    // +optional
    Attestations []AttestationStatus `json:"attestations,omitempty"`

    // ObservedGeneration is the generation observed by the controller
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type AttestationStatus struct {
    // Type is the attestation type (e.g., "vulnerability-scan", "stig-compliance")
    Type string `json:"type"`
    // Verified indicates if the attestation is valid
    Verified bool `json:"verified"`
    // VerifiedAt is when the attestation was verified
    VerifiedAt *metav1.Time `json:"verifiedAt,omitempty"`
    // Message contains additional information
    Message string `json:"message,omitempty"`
}
```

**Acceptance Criteria:**
- [ ] Types compile successfully
- [ ] DeepCopy generated without errors
- [ ] CRD manifest is valid and applies to cluster
- [ ] JSON marshaling/unmarshaling works correctly
- [ ] Printer columns display correctly with `kubectl get catalogitems`

---

### Slice 2.4: ClusterCatalogItem Type

**File**: `pkg/apis/solar/v1alpha1/clustercatalogitem_types.go`

**Tasks:**
- [ ] Define ClusterCatalogItem (cluster-scoped variant)
- [ ] Same spec as CatalogItem but cluster-scoped
- [ ] Add cluster scope marker
- [ ] Run code generation

**Type definition:**
```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Component",type=string,JSONPath=`.spec.component.name`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.component.version`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterCatalogItem represents a cluster-scoped OCM package
type ClusterCatalogItem struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   CatalogItemSpec   `json:"spec,omitempty"`
    Status CatalogItemStatus `json:"status,omitempty"`
}
```

**Acceptance Criteria:**
- [ ] CRD has `scope: Cluster`
- [ ] Resource can be created without namespace
- [ ] RBAC rules generated correctly for cluster-scoped access

---

### Slice 2.5: ClusterRegistration Type

**File**: `pkg/apis/solar/v1alpha1/clusterregistration_types.go`

**Tasks:**
- [ ] Define ClusterRegistration struct
- [ ] Include agent configuration in spec
- [ ] Include cluster capacity and capability fields
- [ ] Include security domain configuration
- [ ] Run code generation

**Type definition:**
```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="K8s Version",type=string,JSONPath=`.status.kubernetesVersion`
// +kubebuilder:printcolumn:name="Last Heartbeat",type=date,JSONPath=`.status.lastHeartbeat`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ClusterRegistration represents a registered target cluster
type ClusterRegistration struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   ClusterRegistrationSpec   `json:"spec,omitempty"`
    Status ClusterRegistrationStatus `json:"status,omitempty"`
}

type ClusterRegistrationSpec struct {
    // DisplayName is a human-readable name for the cluster
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    // +kubebuilder:validation:MaxLength=253
    DisplayName string `json:"displayName"`

    // Description of the cluster purpose
    // +optional
    Description string `json:"description,omitempty"`

    // Labels for cluster categorization
    // +optional
    Labels map[string]string `json:"labels,omitempty"`

    // SecurityDomain identifies the security domain this cluster operates in
    // +optional
    SecurityDomain string `json:"securityDomain,omitempty"`

    // RequiredAttestations lists attestation types required for deployments
    // +optional
    RequiredAttestations []string `json:"requiredAttestations,omitempty"`

    // AgentConfig holds configuration for the solar-agent
    // +optional
    AgentConfig AgentConfiguration `json:"agentConfig,omitempty"`

    // Capacity describes the cluster's resource capacity
    // +optional
    Capacity ClusterCapacity `json:"capacity,omitempty"`

    // Capabilities lists features available on the cluster
    // +optional
    Capabilities []string `json:"capabilities,omitempty"`

    // Paused suspends reconciliation for this cluster
    // +optional
    Paused bool `json:"paused,omitempty"`
}

type AgentConfiguration struct {
    // SourceRegistry is the OCI registry URL for pulling artifacts
    // +kubebuilder:validation:Required
    SourceRegistry string `json:"sourceRegistry"`

    // PollingInterval is how often the agent polls for updates
    // +kubebuilder:default="5m"
    // +optional
    PollingInterval metav1.Duration `json:"pollingInterval,omitempty"`

    // SyncEnabled allows the agent to create Sync resources for catalog chaining
    // +optional
    SyncEnabled bool `json:"syncEnabled,omitempty"`

    // ARCEndpoint is the ARC endpoint for catalog chaining
    // +optional
    ARCEndpoint string `json:"arcEndpoint,omitempty"`
}

type ClusterCapacity struct {
    // CPU capacity in cores
    // +optional
    CPU resource.Quantity `json:"cpu,omitempty"`

    // Memory capacity
    // +optional
    Memory resource.Quantity `json:"memory,omitempty"`

    // Storage capacity
    // +optional
    Storage resource.Quantity `json:"storage,omitempty"`

    // GPUs available
    // +optional
    GPUs int `json:"gpus,omitempty"`
}

// ClusterPhase represents the cluster lifecycle phase
// +kubebuilder:validation:Enum=Pending;Connecting;Connected;Disconnected;Error
type ClusterPhase string

const (
    ClusterPhasePending      ClusterPhase = "Pending"
    ClusterPhaseConnecting   ClusterPhase = "Connecting"
    ClusterPhaseConnected    ClusterPhase = "Connected"
    ClusterPhaseDisconnected ClusterPhase = "Disconnected"
    ClusterPhaseError        ClusterPhase = "Error"
)

type ClusterRegistrationStatus struct {
    // Phase indicates the current cluster state
    // +optional
    Phase ClusterPhase `json:"phase,omitempty"`

    // Conditions represent the latest available observations
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // AgentVersion is the version of the connected agent
    // +optional
    AgentVersion string `json:"agentVersion,omitempty"`

    // KubernetesVersion of the cluster
    // +optional
    KubernetesVersion string `json:"kubernetesVersion,omitempty"`

    // LastHeartbeat is when the agent last reported
    // +optional
    LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

    // InstalledReleases lists releases deployed to this cluster
    // +optional
    InstalledReleases []ReleaseReference `json:"installedReleases,omitempty"`

    // AllocatedCapacity shows resources consumed by releases
    // +optional
    AllocatedCapacity ClusterCapacity `json:"allocatedCapacity,omitempty"`

    // AgentCredentials contains the reference to agent credentials secret
    // +optional
    AgentCredentialsRef *ObjectReference `json:"agentCredentialsRef,omitempty"`

    // ObservedGeneration is the generation observed by the controller
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type ReleaseReference struct {
    // Name of the Release
    Name string `json:"name"`
    // Namespace of the Release
    Namespace string `json:"namespace"`
    // Phase of the Release
    Phase string `json:"phase"`
}
```

**Acceptance Criteria:**
- [ ] Types compile successfully
- [ ] CRD manifest is valid
- [ ] Capacity fields use proper Kubernetes quantity format
- [ ] Agent configuration includes all necessary fields

---

### Slice 2.6: Release Type

**File**: `pkg/apis/solar/v1alpha1/release_types.go`

**Tasks:**
- [ ] Define Release struct linking catalog items to clusters
- [ ] Include values override capability
- [ ] Include suspend mechanism and scaling parameters
- [ ] Add validation for cross-namespace references
- [ ] Run code generation

**Type definition:**
```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Catalog Item",type=string,JSONPath=`.spec.catalogItemRef.name`
// +kubebuilder:printcolumn:name="Target Cluster",type=string,JSONPath=`.spec.targetClusterRef.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.appliedVersion`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Release represents a deployment of a catalog item to a cluster
type Release struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   ReleaseSpec   `json:"spec,omitempty"`
    Status ReleaseStatus `json:"status,omitempty"`
}

type ReleaseSpec struct {
    // CatalogItemRef references the catalog item to deploy
    // +kubebuilder:validation:Required
    CatalogItemRef ObjectReference `json:"catalogItemRef"`

    // TargetClusterRef references the target cluster
    // +kubebuilder:validation:Required
    TargetClusterRef ObjectReference `json:"targetClusterRef"`

    // Version specifies the component version to deploy
    // If empty, uses the latest available version
    // +optional
    Version string `json:"version,omitempty"`

    // Values are the configuration values for the release
    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    Values *runtime.RawExtension `json:"values,omitempty"`

    // Scaling configures resource scaling for the release
    // +optional
    Scaling *ScalingConfig `json:"scaling,omitempty"`

    // Suspend prevents reconciliation if true
    // +optional
    Suspend bool `json:"suspend,omitempty"`

    // DependsOn lists other releases that must be ready first
    // +optional
    DependsOn []ObjectReference `json:"dependsOn,omitempty"`
}

type ScalingConfig struct {
    // MaxUsers is the expected maximum number of users
    // Used to determine scaling parameters
    // +optional
    MaxUsers int `json:"maxUsers,omitempty"`

    // ResourceMultiplier scales the base resource requirements
    // +kubebuilder:validation:Minimum=0.1
    // +kubebuilder:validation:Maximum=10
    // +kubebuilder:default=1
    // +optional
    ResourceMultiplier string `json:"resourceMultiplier,omitempty"`
}

// ReleasePhase represents the release lifecycle phase
// +kubebuilder:validation:Enum=Pending;PreflightChecking;Rendering;Applying;Ready;Failed;Suspended
type ReleasePhase string

const (
    ReleasePhasePending           ReleasePhase = "Pending"
    ReleasePhasePreflightChecking ReleasePhase = "PreflightChecking"
    ReleasePhaseRendering         ReleasePhase = "Rendering"
    ReleasePhaseApplying          ReleasePhase = "Applying"
    ReleasePhaseReady             ReleasePhase = "Ready"
    ReleasePhaseFailed            ReleasePhase = "Failed"
    ReleasePhaseSuspended         ReleasePhase = "Suspended"
)

type ReleaseStatus struct {
    // Phase indicates the current release state
    // +optional
    Phase ReleasePhase `json:"phase,omitempty"`

    // Conditions represent the latest available observations
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // AppliedVersion is the currently applied component version
    // +optional
    AppliedVersion string `json:"appliedVersion,omitempty"`

    // AppliedValuesHash is the hash of applied configuration values
    // +optional
    AppliedValuesHash string `json:"appliedValuesHash,omitempty"`

    // RenderedArtifactRef points to the OCI artifact containing rendered manifests
    // +optional
    RenderedArtifactRef string `json:"renderedArtifactRef,omitempty"`

    // LastAppliedTime is when the release was last applied
    // +optional
    LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`

    // PreflightResults contains results of preflight checks
    // +optional
    PreflightResults []PreflightResult `json:"preflightResults,omitempty"`

    // ObservedGeneration is the generation observed by the controller
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`

    // FailureReason contains a brief reason for failure
    // +optional
    FailureReason string `json:"failureReason,omitempty"`

    // FailureMessage contains a detailed failure message
    // +optional
    FailureMessage string `json:"failureMessage,omitempty"`
}

type PreflightResult struct {
    // Check is the name of the preflight check
    Check string `json:"check"`
    // Passed indicates if the check passed
    Passed bool `json:"passed"`
    // Message contains additional information
    Message string `json:"message,omitempty"`
}
```

**Acceptance Criteria:**
- [ ] Types compile successfully
- [ ] CRD manifest is valid
- [ ] Values field accepts arbitrary JSON/YAML
- [ ] Phase transitions are validated by enum

---

### Slice 2.7: Sync Type (Catalog Chaining)

**File**: `pkg/apis/solar/v1alpha1/sync_types.go`

**Tasks:**
- [ ] Define Sync struct for catalog chaining feature
- [ ] Include source/destination registry references
- [ ] Include filter rules for selective sync
- [ ] Run code generation

**Type definition:**
```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.source.registry`
// +kubebuilder:printcolumn:name="Destination",type=string,JSONPath=`.spec.destination.registry`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Last Sync",type=date,JSONPath=`.status.lastSyncTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Sync represents a catalog chaining configuration
type Sync struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   SyncSpec   `json:"spec,omitempty"`
    Status SyncStatus `json:"status,omitempty"`
}

type SyncSpec struct {
    // Source defines where to sync from
    // +kubebuilder:validation:Required
    Source SyncEndpoint `json:"source"`

    // Destination defines where to sync to
    // +kubebuilder:validation:Required
    Destination SyncEndpoint `json:"destination"`

    // Filters define which components to sync
    // +optional
    Filters []SyncFilter `json:"filters,omitempty"`

    // Interval defines how often to sync
    // +kubebuilder:default="1h"
    // +optional
    Interval metav1.Duration `json:"interval,omitempty"`

    // Suspend prevents sync if true
    // +optional
    Suspend bool `json:"suspend,omitempty"`
}

type SyncEndpoint struct {
    // Registry is the OCI registry URL
    // +kubebuilder:validation:Required
    Registry string `json:"registry"`

    // CredentialsRef references a secret with registry credentials
    // +optional
    CredentialsRef *ObjectReference `json:"credentialsRef,omitempty"`
}

type SyncFilter struct {
    // Type is the filter type: include or exclude
    // +kubebuilder:validation:Enum=include;exclude
    Type string `json:"type"`

    // Pattern is a glob pattern for component names
    // +optional
    Pattern string `json:"pattern,omitempty"`

    // Labels to match
    // +optional
    Labels map[string]string `json:"labels,omitempty"`
}

// SyncPhase represents the sync lifecycle phase
// +kubebuilder:validation:Enum=Pending;Syncing;Synced;Failed
type SyncPhase string

const (
    SyncPhasePending SyncPhase = "Pending"
    SyncPhaseSyncing SyncPhase = "Syncing"
    SyncPhaseSynced  SyncPhase = "Synced"
    SyncPhaseFailed  SyncPhase = "Failed"
)

type SyncStatus struct {
    // Phase indicates the current sync state
    // +optional
    Phase SyncPhase `json:"phase,omitempty"`

    // Conditions represent the latest available observations
    // +optional
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // LastSyncTime is when the last sync completed
    // +optional
    LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

    // SyncedComponents lists successfully synced components
    // +optional
    SyncedComponents []string `json:"syncedComponents,omitempty"`

    // FailedComponents lists components that failed to sync
    // +optional
    FailedComponents []SyncFailure `json:"failedComponents,omitempty"`

    // ARCOrderRef references the created ARC Order
    // +optional
    ARCOrderRef *ObjectReference `json:"arcOrderRef,omitempty"`

    // ObservedGeneration is the generation observed by the controller
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

type SyncFailure struct {
    // Component that failed
    Component string `json:"component"`
    // Reason for failure
    Reason string `json:"reason"`
}
```

**Acceptance Criteria:**
- [ ] Types compile successfully
- [ ] CRD manifest is valid
- [ ] Filter patterns support glob syntax
- [ ] Sync intervals are validated

---

### Slice 2.8: Generate Clientsets & Register Types

**Files to create:**
```
pkg/client/
  clientset/
    versioned/
      (generated)
  informers/
    (generated)
  listers/
    (generated)
hack/
  update-codegen.sh (update)
```

**Tasks:**
- [ ] Configure client-gen for typed clientset generation
- [ ] Configure informer-gen for informer generation
- [ ] Configure lister-gen for lister generation
- [ ] Create convenience functions for common operations
- [ ] Write integration tests against envtest

**Acceptance Criteria:**
- [ ] `make generate` produces all client code
- [ ] Clientset compiles and can connect to cluster
- [ ] Informers and listers work correctly
- [ ] Integration tests pass against envtest

---

## Phase 3: solar-index Core (Extension APIServer)

**Goal**: Implement the Kubernetes extension apiserver skeleton.

**Test Coverage Target**: 80%+ for API logic

**Status**: ✅ APIServer implemented using `go.opendefense.cloud/kit/apiserver`

**Note**: The implementation uses `solar-apiserver` (not `solar-index`) and leverages the simplified builder pattern from `go.opendefense.cloud/kit/apiserver` rather than the raw k8s.io/apiserver libraries.

### Slice 3.1: APIServer Bootstrap ✅ COMPLETE

**Existing files:**

```text
cmd/solar-apiserver/
  main.go            # Uses apiserver.NewBuilder()
  apiserver_test.go
  suite_test.go
```

**Completed Tasks:**

- [x] Create main.go with apiserver builder pattern
- [x] Set up extension apiserver using `go.opendefense.cloud/kit/apiserver`
- [x] Configure TLS certificate handling (via Helm/cert-manager)
- [x] Health (`/healthz`) and readiness (`/readyz`) endpoints included

**Existing Implementation:**

```go
func main() {
    code := apiserver.NewBuilder(scheme).
        WithComponentName(componentName).
        WithOpenAPIDefinitions(componentName, "v0.1.0", openapi.GetOpenAPIDefinitions).
        With(apiserver.Resource(&solar.CatalogItem{}, solarv1alpha1.SchemeGroupVersion)).
        Execute()
    os.Exit(code)
}
```

**Remaining Tasks:**

- [ ] Add resources for ClusterCatalogItem, ClusterRegistration, Release, Sync
- [ ] Configure metrics endpoint (`/metrics`) if not already present

**Server options structure:**
```go
type SolarIndexOptions struct {
    // RecommendedOptions from apiserver-kit
    RecommendedOptions *genericoptions.RecommendedOptions

    // StdOut and StdErr for command output
    StdOut io.Writer
    StdErr io.Writer
}

func NewSolarIndexOptions(out, err io.Writer) *SolarIndexOptions
func (o *SolarIndexOptions) Validate() []error
func (o *SolarIndexOptions) Complete() error
func (o *SolarIndexOptions) Config() (*apiserver.Config, error)
func (o *SolarIndexOptions) RunServer(ctx context.Context) error
```

**Acceptance Criteria:**
- [ ] Server starts and binds to configured port
- [ ] `/healthz` returns 200 OK
- [ ] `/readyz` returns 200 OK when ready
- [ ] `/metrics` returns Prometheus metrics
- [ ] Server shuts down gracefully on SIGTERM

---

### Slice 3.2: API Group Installation

**Files to create:**
```
internal/index/
  apiserver/
    apiserver.go
    scheme.go
    install.go
```

**Tasks:**
- [ ] Create scheme with all solar.opendefense.cloud types
- [ ] Implement API group installation
- [ ] Configure version priority (v1alpha1)
- [ ] Register conversion functions (identity for v1alpha1)

**Acceptance Criteria:**
- [ ] API group appears in `/apis`
- [ ] Resources appear in `/apis/solar.opendefense.cloud/v1alpha1`
- [ ] OpenAPI spec includes all types

---

### Slice 3.3: REST Storage - CatalogItem

**Files to create:**
```
internal/index/
  registry/
    catalogitem/
      strategy.go
      storage.go
      status_strategy.go
```

**Tasks:**
- [ ] Implement `rest.Storage` interface for CatalogItem
- [ ] Implement create/update strategies with validation
- [ ] Implement status subresource strategy
- [ ] Add field selectors for common queries
- [ ] Add label selectors
- [ ] Implement table conversion for `kubectl get`

**REST storage interface:**
```go
type CatalogItemStorage struct {
    CatalogItem *REST
    Status      *StatusREST
}

func NewStorage(scheme *runtime.Scheme, optsGetter generic.RESTOptionsGetter) (*CatalogItemStorage, error)
```

**Acceptance Criteria:**
- [ ] CRUD operations work via kubectl
- [ ] Status updates don't modify spec
- [ ] Field selectors filter correctly
- [ ] Table output shows correct columns

---

### Slice 3.4: REST Storage - ClusterRegistration

**Files to create:**
```
internal/index/
  registry/
    clusterregistration/
      strategy.go
      storage.go
      status_strategy.go
      credentials_rest.go
```

**Tasks:**
- [ ] Implement REST storage for ClusterRegistration
- [ ] Implement `/credentials` subresource for agent credential retrieval
- [ ] Generate agent credentials on create
- [ ] Add finalizer for cleanup

**Acceptance Criteria:**
- [ ] CRUD operations work
- [ ] Credentials subresource returns agent configuration
- [ ] Finalizer prevents deletion until cleanup

---

### Slice 3.5: REST Storage - Release

**Files to create:**
```
internal/index/
  registry/
    release/
      strategy.go
      storage.go
      status_strategy.go
```

**Tasks:**
- [ ] Implement REST storage for Release
- [ ] Validate catalog item and cluster references exist
- [ ] Implement status subresource

**Acceptance Criteria:**
- [ ] CRUD operations work
- [ ] Invalid references are rejected
- [ ] Status updates work correctly

---

### Slice 3.6: REST Storage - Sync

**Files to create:**
```
internal/index/
  registry/
    sync/
      strategy.go
      storage.go
      status_strategy.go
```

**Tasks:**
- [ ] Implement REST storage for Sync
- [ ] Validate registry URLs
- [ ] Implement status subresource

**Acceptance Criteria:**
- [ ] CRUD operations work
- [ ] Invalid URLs are rejected
- [ ] Status updates work correctly

---

### Slice 3.7: REST Storage - ClusterCatalogItem

**Files to create:**
```
internal/index/
  registry/
    clustercatalogitem/
      strategy.go
      storage.go
      status_strategy.go
```

**Tasks:**
- [ ] Implement REST storage for cluster-scoped CatalogItem
- [ ] Reuse strategies from namespaced CatalogItem where possible

**Acceptance Criteria:**
- [ ] CRUD operations work without namespace
- [ ] RBAC enforces cluster-admin for mutations

---

### Slice 3.8: Authentication Integration

**Files to create:**
```
internal/index/
  auth/
    authenticator.go
    delegating_authenticator.go
```

**Tasks:**
- [ ] Configure delegating authentication to kube-apiserver
- [ ] Support ServiceAccount token authentication
- [ ] Support OIDC token authentication (pass-through)
- [ ] Add authentication metrics

**Acceptance Criteria:**
- [ ] ServiceAccount tokens authenticate correctly
- [ ] Anonymous requests are rejected (unless explicitly allowed)
- [ ] Authentication failures return 401

---

### Slice 3.9: Authorization Integration

**Files to create:**
```
internal/index/
  auth/
    authorizer.go
    delegating_authorizer.go
```

**Tasks:**
- [ ] Configure delegating authorization to kube-apiserver
- [ ] Support SubjectAccessReview for authorization
- [ ] Add authorization metrics
- [ ] Configure audit logging

**Acceptance Criteria:**
- [ ] RBAC rules are enforced
- [ ] Unauthorized requests return 403
- [ ] Audit logs capture all requests

---

### Slice 3.10: OpenAPI Schema Generation

**Files to create:**
```
api/
  openapi/
    generated.openapi.go
hack/
  update-openapi.sh
```

**Tasks:**
- [ ] Configure OpenAPI v3 schema generation
- [ ] Add validation rules via OpenAPI
- [ ] Serve OpenAPI spec at `/openapi/v3`
- [ ] Generate TypeScript types from OpenAPI for frontend

**Acceptance Criteria:**
- [ ] OpenAPI spec is valid
- [ ] Schema includes all validation rules
- [ ] TypeScript types compile without errors

---

## Phase 4: solar-index Storage & Persistence

**Goal**: Implement persistent storage backend.

**Test Coverage Target**: 80%+

### Slice 4.1: etcd Storage Backend

**Files to create:**
```
internal/index/
  storage/
    etcd/
      factory.go
      options.go
      health.go
```

**Tasks:**
- [ ] Configure etcd3 storage backend
- [ ] Implement storage factory for all resource types
- [ ] Add connection pooling
- [ ] Configure TLS for etcd communication
- [ ] Add retry logic with exponential backoff

**Acceptance Criteria:**
- [ ] Data persists across server restarts
- [ ] TLS connections work correctly
- [ ] Connection failures are retried

---

### Slice 4.2: Watch Support

**Tasks:**
- [ ] Verify watch works for all resource types
- [ ] Add bookmark support for efficient watches
- [ ] Configure watch cache size
- [ ] Test watch reconnection scenarios

**Acceptance Criteria:**
- [ ] Watches receive create/update/delete events
- [ ] Bookmarks reduce reconnection traffic
- [ ] Watch cache improves performance

---

### Slice 4.3: Storage Metrics

**Tasks:**
- [ ] Add etcd request latency metrics
- [ ] Add storage operation counters
- [ ] Add watch metrics (active watches, events)
- [ ] Export via OpenTelemetry

**Acceptance Criteria:**
- [ ] Metrics appear in `/metrics` endpoint
- [ ] Latency percentiles are accurate
- [ ] Watch metrics track active connections

---

## Phase 5: solar-index Validation & Admission

**Goal**: Implement validation and mutation logic.

**Test Coverage Target**: 85%+

### Slice 5.1: Validation Framework

**Files to create:**
```
internal/index/
  admission/
    validator.go
    registry.go
```

**Tasks:**
- [ ] Create validation registry for all types
- [ ] Implement validation interface
- [ ] Add detailed error messages with field paths

**Acceptance Criteria:**
- [ ] Invalid resources are rejected with clear errors
- [ ] Error messages include field paths
- [ ] Validation runs on create and update

---

### Slice 5.2: CatalogItem Validation

**Files to create:**
```
internal/index/
  admission/
    catalogitem_validator.go
    catalogitem_validator_test.go
```

**Tasks:**
- [ ] Validate OCM component reference format
- [ ] Validate semantic version format
- [ ] Validate repository URL format
- [ ] Validate values schema is valid JSON Schema
- [ ] Check for duplicate component+version combinations

**Validation rules:**
```go
func ValidateCatalogItemSpec(spec *v1alpha1.CatalogItemSpec, fldPath *field.Path) field.ErrorList {
    var errs field.ErrorList

    // Component name must be valid OCM component name
    if !isValidOCMComponentName(spec.Component.Name) {
        errs = append(errs, field.Invalid(fldPath.Child("component", "name"),
            spec.Component.Name, "must be a valid OCM component name"))
    }

    // Version must be valid semver
    if _, err := semver.Parse(spec.Component.Version); err != nil {
        errs = append(errs, field.Invalid(fldPath.Child("component", "version"),
            spec.Component.Version, "must be a valid semantic version"))
    }

    // Repository must be valid URL
    if _, err := url.Parse(spec.Component.Repository); err != nil {
        errs = append(errs, field.Invalid(fldPath.Child("component", "repository"),
            spec.Component.Repository, "must be a valid URL"))
    }

    return errs
}
```

**Acceptance Criteria:**
- [ ] Invalid component names are rejected
- [ ] Invalid versions are rejected
- [ ] Invalid URLs are rejected
- [ ] Duplicate detection works
- [ ] Test coverage 85%+

---

### Slice 5.3: ClusterRegistration Validation & Mutation

**Files to create:**
```
internal/index/
  admission/
    clusterregistration_validator.go
    clusterregistration_mutator.go
    clusterregistration_test.go
```

**Tasks:**
- [ ] Validate security domain is in allowed list
- [ ] Validate registry URL format
- [ ] Generate unique agent credentials on create (mutation)
- [ ] Set default polling interval (mutation)
- [ ] Add finalizer for cleanup (mutation)

**Acceptance Criteria:**
- [ ] Invalid security domains are rejected
- [ ] Agent credentials are generated automatically
- [ ] Defaults are applied correctly
- [ ] Test coverage 85%+

---

### Slice 5.4: Release Validation

**Files to create:**
```
internal/index/
  admission/
    release_validator.go
    release_validator_test.go
```

**Tasks:**
- [ ] Validate catalog item reference exists
- [ ] Validate cluster registration reference exists
- [ ] Validate values against catalog item's values schema
- [ ] Validate version exists in catalog item's available versions
- [ ] Prevent conflicting releases (same catalog item + cluster)

**Acceptance Criteria:**
- [ ] Missing references are rejected
- [ ] Invalid values are rejected
- [ ] Duplicate releases are rejected
- [ ] Test coverage 85%+

---

### Slice 5.5: Sync Validation

**Files to create:**
```
internal/index/
  admission/
    sync_validator.go
    sync_validator_test.go
```

**Tasks:**
- [ ] Validate source and destination registry URLs
- [ ] Validate filter patterns are valid globs
- [ ] Validate interval is reasonable (min 5m, max 24h)

**Acceptance Criteria:**
- [ ] Invalid URLs are rejected
- [ ] Invalid patterns are rejected
- [ ] Unreasonable intervals are rejected
- [ ] Test coverage 85%+

---

## Phase 6: solar-discovery Component

**Goal**: Implement OCI registry scanner for OCM packages.

**Test Coverage Target**: 80%+

### Slice 6.1: OCI Registry Client

**Files to create:**
```
pkg/registry/
  client.go
  oci/
    client.go
    auth.go
    options.go
  client_test.go
  testutil/
    mock_registry.go
```

**Tasks:**
- [ ] Define registry client interface
- [ ] Implement OCI registry client using oras-go v2.6.0
- [ ] Support authentication methods:
  - Basic auth
  - Bearer token
  - Docker config keychain
- [ ] Add retry and timeout handling
- [ ] Add connection pooling
- [ ] Write unit tests with mock registry

**Interface definition:**
```go
type Client interface {
    // ListRepositories lists all repositories in the registry
    ListRepositories(ctx context.Context) ([]string, error)

    // ListTags lists all tags for a repository
    ListTags(ctx context.Context, repository string) ([]string, error)

    // GetManifest retrieves a manifest by reference
    GetManifest(ctx context.Context, repository, reference string) (ocispec.Manifest, error)

    // GetBlob retrieves a blob by digest
    GetBlob(ctx context.Context, repository string, digest digest.Digest) (io.ReadCloser, error)

    // PushManifest pushes a manifest
    PushManifest(ctx context.Context, repository, reference string, manifest ocispec.Manifest) error

    // PushBlob pushes a blob
    PushBlob(ctx context.Context, repository string, blob io.Reader) (digest.Digest, error)
}

type ClientOptions struct {
    // Timeout for operations
    Timeout time.Duration
    // RetryCount for failed operations
    RetryCount int
    // RetryDelay between retries
    RetryDelay time.Duration
    // Insecure allows HTTP connections
    Insecure bool
}
```

**Acceptance Criteria:**
- [ ] Can list repositories from OCI registry
- [ ] Can list tags for a repository
- [ ] Can retrieve manifests and blobs
- [ ] Authentication works with all methods
- [ ] Retries handle transient failures
- [ ] Test coverage 80%+

---

### Slice 6.2: OCM Package Scanner

**Files to create:**
```
pkg/ocm/
  scanner.go
  parser.go
  types.go
  scanner_test.go
```

**Tasks:**
- [ ] Implement OCM component descriptor parser
- [ ] Detect solar-compatible packages via labels/annotations
- [ ] Extract metadata for catalog items
- [ ] Handle version discovery
- [ ] Parse resource references

**Interface definition:**
```go
type Scanner interface {
    // ScanRepository scans an OCI repository for OCM components
    ScanRepository(ctx context.Context, registry, repository string) ([]Component, error)

    // GetComponentDescriptor retrieves and parses a component descriptor
    GetComponentDescriptor(ctx context.Context, ref ComponentReference) (*ComponentDescriptor, error)

    // ListVersions lists all available versions of a component
    ListVersions(ctx context.Context, registry, componentName string) ([]string, error)
}

type Component struct {
    Name         string
    Version      string
    Repository   string
    DisplayName  string
    Description  string
    Labels       map[string]string
    Dependencies []ComponentReference
    Resources    []Resource
    Sources      []Source
}

// IsSolarCompatible checks if component has solar compatibility label
func (c *Component) IsSolarCompatible() bool {
    return c.Labels["solar.opendefense.cloud/catalog"] == "true"
}
```

**Acceptance Criteria:**
- [ ] Can parse OCM component descriptors
- [ ] Correctly identifies solar-compatible components
- [ ] Extracts all required metadata
- [ ] Handles multiple versions
- [ ] Test coverage 80%+

---

### Slice 6.3: Discovery Controller Setup

**Files to create:**
```
cmd/solar-discovery/
  main.go
internal/discovery/
  app/
    server.go
    options.go
  config/
    config.go
```

**Tasks:**
- [ ] Create main.go entry point
- [ ] Implement controller-runtime manager setup
- [ ] Configure leader election for HA
- [ ] Add health and metrics endpoints
- [ ] Load configuration from ConfigMap/Secret

**Configuration:**
```go
type DiscoveryConfig struct {
    // Registries to scan
    Registries []RegistryConfig `yaml:"registries"`

    // ScanInterval between full scans
    ScanInterval time.Duration `yaml:"scanInterval"`

    // Namespace to create CatalogItems in (empty for cluster-scoped)
    TargetNamespace string `yaml:"targetNamespace"`

    // LeaderElection configuration
    LeaderElection LeaderElectionConfig `yaml:"leaderElection"`
}

type RegistryConfig struct {
    // URL of the registry
    URL string `yaml:"url"`

    // CredentialsSecretRef for registry authentication
    CredentialsSecretRef string `yaml:"credentialsSecretRef,omitempty"`

    // Repositories to scan (empty for all)
    Repositories []string `yaml:"repositories,omitempty"`

    // ScanInterval for this specific registry
    ScanInterval time.Duration `yaml:"scanInterval,omitempty"`
}
```

**Acceptance Criteria:**
- [ ] Controller starts and initializes correctly
- [ ] Leader election works with multiple replicas
- [ ] Configuration loads from ConfigMap
- [ ] Metrics are exposed

---

### Slice 6.4: Registry Scan Reconciler

**Files to create:**
```
internal/discovery/
  controller/
    registry_controller.go
    registry_controller_test.go
```

**Tasks:**
- [ ] Implement periodic registry scanning
- [ ] Track scan progress and status
- [ ] Handle registry authentication
- [ ] Add observability (tracing, metrics)

**Acceptance Criteria:**
- [ ] Registries are scanned at configured intervals
- [ ] Scan progress is tracked
- [ ] Errors are logged and don't stop other scans
- [ ] Test coverage 80%+

---

### Slice 6.5: CatalogItem Synchronization

**Files to create:**
```
internal/discovery/
  controller/
    catalogitem_sync.go
    catalogitem_sync_test.go
```

**Tasks:**
- [ ] Create CatalogItems for discovered components
- [ ] Update existing CatalogItems on changes
- [ ] Handle deleted components (mark as unavailable/archived)
- [ ] Respect namespace boundaries
- [ ] Handle version updates

**Acceptance Criteria:**
- [ ] New components create CatalogItems
- [ ] Updated components update CatalogItems
- [ ] Removed components are marked archived
- [ ] Multiple versions are tracked
- [ ] Test coverage 80%+

---

### Slice 6.6: Discovery Observability

**Tasks:**
- [ ] Add metrics:
  - `solar_discovery_scan_duration_seconds`
  - `solar_discovery_items_discovered_total`
  - `solar_discovery_scan_errors_total`
  - `solar_discovery_registries_configured`
- [ ] Add tracing spans for registry operations
- [ ] Implement structured logging with context
- [ ] Create Grafana dashboard template

**Acceptance Criteria:**
- [ ] All metrics are exported
- [ ] Traces show scan flow
- [ ] Logs include trace IDs
- [ ] Dashboard template works

---

## Phase 7: solar-renderer Component

**Goal**: Implement manifest renderer that watches releases and updates OCI images.

**Test Coverage Target**: 80%+

### Slice 7.1: Renderer Controller Setup

**Files to create:**
```
cmd/solar-renderer/
  main.go
internal/renderer/
  app/
    server.go
    options.go
  config/
    config.go
```

**Tasks:**
- [ ] Create main.go entry point
- [ ] Implement controller-runtime manager setup
- [ ] Configure to watch Release resources
- [ ] Add leader election
- [ ] Add health and metrics endpoints

**Acceptance Criteria:**
- [ ] Controller starts successfully
- [ ] Watches Release resources
- [ ] Leader election works

---

### Slice 7.2: Release Reconciler

**Files to create:**
```
internal/renderer/
  controller/
    release_controller.go
    release_controller_test.go
```

**Tasks:**
- [ ] Implement reconciler for Release resources
- [ ] Handle release create/update/delete events
- [ ] Update release status during processing
- [ ] Implement requeue with backoff for errors

**Reconciliation flow:**
```go
func (r *ReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch Release
    // 2. Check if suspended → return early
    // 3. Run preflight checks
    // 4. Render manifests
    // 5. Publish to OCI registry
    // 6. Update status
}
```

**Acceptance Criteria:**
- [ ] Reconciler processes releases
- [ ] Status updates reflect progress
- [ ] Errors trigger requeue with backoff
- [ ] Test coverage 80%+

---

### Slice 7.3: Preflight Check Engine

**Files to create:**
```
internal/renderer/
  preflight/
    engine.go
    checks.go
    checks_test.go
```

**Tasks:**
- [ ] Implement preflight check framework
- [ ] Implement capacity check (fits on cluster)
- [ ] Implement capability check (dependencies met)
- [ ] Implement attestation check (required attestations present)
- [ ] Return detailed check results

**Checks to implement:**
```go
type PreflightChecker interface {
    Name() string
    Check(ctx context.Context, release *v1alpha1.Release, catalogItem *v1alpha1.CatalogItem, cluster *v1alpha1.ClusterRegistration) (PreflightResult, error)
}

// CapacityChecker verifies cluster has sufficient resources
type CapacityChecker struct{}

// CapabilityChecker verifies dependencies are available
type CapabilityChecker struct{}

// AttestationChecker verifies required attestations
type AttestationChecker struct{}
```

**Acceptance Criteria:**
- [ ] All checks are implemented
- [ ] Failed checks block release
- [ ] Check results are recorded in status
- [ ] Test coverage 80%+

---

### Slice 7.4: Manifest Rendering Engine

**Files to create:**
```
internal/renderer/
  engine/
    engine.go
    ocm.go
    values.go
    engine_test.go
```

**Tasks:**
- [ ] Define rendering engine interface
- [ ] Implement OCM component localization
- [ ] Implement values merging (defaults + overrides)
- [ ] Support scaling configuration
- [ ] Handle resource transformations

**Interface:**
```go
type Engine interface {
    // Render renders manifests for a release
    Render(ctx context.Context, release *v1alpha1.Release, catalogItem *v1alpha1.CatalogItem) (*RenderResult, error)
}

type RenderResult struct {
    // Manifests is the list of rendered Kubernetes manifests
    Manifests []unstructured.Unstructured

    // ValuesHash is the hash of applied values
    ValuesHash string

    // ComponentVersion is the rendered component version
    ComponentVersion string
}

// MergeValues merges default values with overrides
func MergeValues(defaults, overrides map[string]any) map[string]any
```

**Acceptance Criteria:**
- [ ] Engine renders valid manifests
- [ ] Values are merged correctly
- [ ] Scaling affects resource requests
- [ ] Test coverage 80%+

---

### Slice 7.5: OCI Image Publisher

**Files to create:**
```
internal/renderer/
  publisher/
    publisher.go
    layering.go
    publisher_test.go
```

**Tasks:**
- [ ] Implement OCI image creation from manifests
- [ ] Add content-addressable layering
- [ ] Implement atomic publish (push then tag)
- [ ] Handle large manifests efficiently
- [ ] Support image signing (future)

**Interface:**
```go
type Publisher interface {
    // Publish publishes rendered manifests as an OCI image
    Publish(ctx context.Context, cluster, release string, manifests []unstructured.Unstructured) (string, error)
}

// ImageRef returns the full image reference
// Format: {registry}/{namespace}/desired-state/{cluster}:{release-hash}
func ImageRef(registry, namespace, cluster, hash string) string
```

**Acceptance Criteria:**
- [ ] Manifests are published as OCI images
- [ ] Images are content-addressable
- [ ] Publish is atomic
- [ ] Test coverage 80%+

---

### Slice 7.6: Desired State Management

**Files to create:**
```
internal/renderer/
  state/
    manager.go
    differ.go
    manager_test.go
```

**Tasks:**
- [ ] Define desired state structure per cluster
- [ ] Implement state diffing
- [ ] Track which releases contribute to state
- [ ] Handle release removal (cleanup)

**Acceptance Criteria:**
- [ ] State reflects active releases
- [ ] Removed releases update state
- [ ] Diff is accurate
- [ ] Test coverage 80%+

---

### Slice 7.7: Renderer Observability

**Tasks:**
- [ ] Add metrics:
  - `solar_renderer_render_duration_seconds`
  - `solar_renderer_publish_duration_seconds`
  - `solar_renderer_releases_processed_total`
  - `solar_renderer_preflight_failures_total`
- [ ] Add tracing for render pipeline
- [ ] Implement structured logging
- [ ] Create Grafana dashboard template

**Acceptance Criteria:**
- [ ] All metrics exported
- [ ] Traces show render flow
- [ ] Dashboard template works

---

## Phase 8: solar-agent Component

**Goal**: Implement cluster agent for status reporting and FluxCD integration.

**Test Coverage Target**: 80%+

### Slice 8.1: Agent Bootstrap

**Files to create:**
```
cmd/solar-agent/
  main.go
internal/agent/
  app/
    server.go
    options.go
  config/
    config.go
    credentials.go
```

**Tasks:**
- [ ] Create agent entrypoint
- [ ] Load configuration from Secret/ConfigMap
- [ ] Establish connection to solar-index API
- [ ] Implement credential refresh
- [ ] Add health endpoints

**Configuration:**
```go
type AgentConfig struct {
    // ClusterName is the name of this cluster in solar-index
    ClusterName string `yaml:"clusterName"`

    // Namespace is the namespace of the ClusterRegistration
    Namespace string `yaml:"namespace"`

    // SolarIndexURL is the URL of the solar-index API
    SolarIndexURL string `yaml:"solarIndexUrl"`

    // SourceRegistry is the OCI registry for desired state
    SourceRegistry string `yaml:"sourceRegistry"`

    // DesiredStateImage is the image containing desired state
    DesiredStateImage string `yaml:"desiredStateImage"`

    // PollingInterval for checking updates
    PollingInterval time.Duration `yaml:"pollingInterval"`

    // SyncEnabled allows Sync resource management
    SyncEnabled bool `yaml:"syncEnabled"`

    // ARCEndpoint for catalog chaining
    ARCEndpoint string `yaml:"arcEndpoint,omitempty"`
}
```

**Acceptance Criteria:**
- [ ] Agent starts and connects to solar-index
- [ ] Configuration loads correctly
- [ ] Health endpoint reports status
- [ ] Credentials are refreshed before expiry

---

### Slice 8.2: Preflight Checks

**Files to create:**
```
internal/agent/
  preflight/
    runner.go
    checks.go
    checks_test.go
```

**Tasks:**
- [ ] Implement preflight check runner
- [ ] Check FluxCD installation
- [ ] Check required CRDs
- [ ] Check RBAC permissions
- [ ] Check network connectivity to OCI registry
- [ ] Report results to solar-index

**Checks:**
```go
type Check interface {
    Name() string
    Run(ctx context.Context) Result
}

type Result struct {
    Passed  bool
    Message string
}

// Checks to implement:
// - FluxCDInstalled: Verify flux-system namespace and controllers
// - RequiredCRDs: Verify OCIRepository, Kustomization CRDs
// - RBACPermissions: Verify agent has required permissions
// - RegistryConnectivity: Verify can pull from source registry
```

**Acceptance Criteria:**
- [ ] All checks implemented
- [ ] Results reported to solar-index
- [ ] Failed checks block agent startup (configurable)
- [ ] Test coverage 80%+

---

### Slice 8.3: FluxCD Resource Management

**Files to create:**
```
internal/agent/
  flux/
    manager.go
    ocirepository.go
    kustomization.go
    manager_test.go
```

**Tasks:**
- [ ] Create OCIRepository pointing to desired state image
- [ ] Create Kustomization to apply manifests
- [ ] Handle updates when desired state changes
- [ ] Implement cleanup on release removal
- [ ] Watch for reconciliation status

**FluxCD resources created:**
```go
// OCIRepository for desired state
func (m *Manager) CreateOCIRepository(ctx context.Context, name, imageRef string) error

// Kustomization to apply manifests
func (m *Manager) CreateKustomization(ctx context.Context, name, sourceRef string) error

// Cleanup removes FluxCD resources
func (m *Manager) Cleanup(ctx context.Context, name string) error
```

**Acceptance Criteria:**
- [ ] OCIRepository created correctly
- [ ] Kustomization applies manifests
- [ ] Updates trigger reconciliation
- [ ] Cleanup removes all resources
- [ ] Test coverage 80%+

---

### Slice 8.4: Status Reporter

**Files to create:**
```
internal/agent/
  status/
    collector.go
    reporter.go
    reporter_test.go
```

**Tasks:**
- [ ] Collect FluxCD reconciliation status
- [ ] Collect deployed resource health
- [ ] Collect cluster capacity/usage
- [ ] Report heartbeat to solar-index
- [ ] Handle solar-index unavailability gracefully

**Status collection:**
```go
type StatusCollector interface {
    // Collect gathers current cluster status
    Collect(ctx context.Context) (*ClusterStatus, error)
}

type ClusterStatus struct {
    // KubernetesVersion of the cluster
    KubernetesVersion string

    // Capacity is the total cluster capacity
    Capacity v1alpha1.ClusterCapacity

    // Allocatable is the allocatable capacity
    Allocatable v1alpha1.ClusterCapacity

    // ReleaseStatuses is the status of each release
    ReleaseStatuses []ReleaseStatus
}

type Reporter interface {
    // Report sends status to solar-index
    Report(ctx context.Context, status *ClusterStatus) error
}
```

**Acceptance Criteria:**
- [ ] Status collected from FluxCD resources
- [ ] Heartbeat sent at configured interval
- [ ] Status appears in ClusterRegistration
- [ ] Handles solar-index downtime
- [ ] Test coverage 80%+

---

### Slice 8.5: Sync Controller (Catalog Chaining)

**Files to create:**
```
internal/agent/
  sync/
    controller.go
    arc.go
    controller_test.go
```

**Tasks:**
- [ ] Watch Sync resources (when sync enabled)
- [ ] Create ARC Order resources
- [ ] Track sync status
- [ ] Handle sync failures

**Acceptance Criteria:**
- [ ] Sync resources trigger ARC orders
- [ ] Status is reported back
- [ ] Failures are handled gracefully
- [ ] Only runs when sync enabled
- [ ] Test coverage 80%+

---

### Slice 8.6: Agent Observability

**Tasks:**
- [ ] Add metrics:
  - `solar_agent_heartbeat_timestamp`
  - `solar_agent_releases_managed`
  - `solar_agent_sync_operations_total`
  - `solar_agent_flux_reconciliations_total`
- [ ] Add tracing for status reporting
- [ ] Implement structured logging
- [ ] Create Grafana dashboard template

**Acceptance Criteria:**
- [ ] All metrics exported
- [ ] Traces show agent operations
- [ ] Dashboard template works

---

## Phase 9: solar-ui Frontend

**Goal**: Implement React/Next.js management UI.

**Test Coverage Target**: 70%+ (unit), E2E for critical flows

### Slice 9.1: Next.js Project Setup

**Files to create:**
```
solar-ui/
  package.json
  tsconfig.json
  next.config.ts
  tailwind.config.ts
  postcss.config.js
  .eslintrc.json
  .prettierrc
  src/
    app/
      layout.tsx
      page.tsx
      globals.css
    lib/
    components/
    hooks/
    types/
```

**Tasks:**
- [ ] Initialize Next.js 16 with App Router
- [ ] Configure TypeScript strict mode
- [ ] Set up TailwindCSS v4
- [ ] Add shadcn/ui components
- [ ] Configure ESLint and Prettier
- [ ] Set up Turbopack for development

**Package.json dependencies:**
```json
{
  "dependencies": {
    "next": "^16.0.0",
    "react": "^19.2.0",
    "react-dom": "^19.2.0",
    "@tanstack/react-query": "^5.x",
    "zod": "^3.x",
    "date-fns": "^4.x"
  },
  "devDependencies": {
    "typescript": "^5.7.0",
    "tailwindcss": "^4.1.0",
    "@types/react": "^19.0.0",
    "@types/node": "^22.0.0",
    "eslint": "^9.x",
    "prettier": "^3.x"
  }
}
```

**Acceptance Criteria:**
- [ ] `npm run dev` starts with Turbopack
- [ ] TypeScript compiles without errors
- [ ] ESLint passes
- [ ] Tailwind classes work

---

### Slice 9.2: Design System & Components

**Files to create:**
```
solar-ui/src/
  components/
    ui/
      (shadcn components)
    layout/
      Header.tsx
      Sidebar.tsx
      Footer.tsx
      PageLayout.tsx
    common/
      LoadingSpinner.tsx
      ErrorBoundary.tsx
      EmptyState.tsx
      StatusBadge.tsx
```

**Tasks:**
- [ ] Install core shadcn/ui components
- [ ] Create layout components
- [ ] Define color scheme and typography
- [ ] Create status badge component
- [ ] Add loading and error states
- [ ] Ensure accessibility (WCAG 2.1 AA)

**Acceptance Criteria:**
- [ ] All components render correctly
- [ ] Dark mode works
- [ ] Components are accessible
- [ ] Consistent styling

---

### Slice 9.3: Kubernetes API Client

**Files to create:**
```
solar-ui/src/
  lib/
    kubernetes/
      client.ts
      types.ts
      errors.ts
    api/
      catalog.ts
      clusters.ts
      releases.ts
      sync.ts
  hooks/
    useCatalogItems.ts
    useClusters.ts
    useReleases.ts
```

**Tasks:**
- [ ] Implement Kubernetes API client using fetch
- [ ] Generate TypeScript types from OpenAPI
- [ ] Create React Query hooks for CRUD operations
- [ ] Handle authentication (token forwarding)
- [ ] Implement error handling

**API client:**
```typescript
// client.ts
export const createKubernetesClient = (baseUrl: string, token: string) => {
  return {
    get: async <T>(path: string): Promise<T> => { ... },
    post: async <T>(path: string, body: unknown): Promise<T> => { ... },
    put: async <T>(path: string, body: unknown): Promise<T> => { ... },
    patch: async <T>(path: string, body: unknown): Promise<T> => { ... },
    delete: async (path: string): Promise<void> => { ... },
    watch: (path: string, onEvent: (event: WatchEvent) => void): () => void => { ... },
  };
};

// hooks/useCatalogItems.ts
export const useCatalogItems = (namespace?: string) => {
  return useQuery({
    queryKey: ['catalogItems', namespace],
    queryFn: () => catalogApi.list(namespace),
  });
};

export const useCatalogItem = (namespace: string, name: string) => {
  return useQuery({
    queryKey: ['catalogItem', namespace, name],
    queryFn: () => catalogApi.get(namespace, name),
  });
};

export const useCreateCatalogItem = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: catalogApi.create,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['catalogItems'] }),
  });
};
```

**Acceptance Criteria:**
- [ ] Can fetch resources from API
- [ ] Watch events update UI
- [ ] Errors are handled gracefully
- [ ] Types match OpenAPI spec

---

### Slice 9.4: Authentication Flow

**Files to create:**
```
solar-ui/src/
  app/
    login/
      page.tsx
    api/
      auth/
        [...nextauth]/
          route.ts
  lib/
    auth/
      config.ts
      provider.tsx
  components/
    auth/
      LoginForm.tsx
      UserMenu.tsx
```

**Tasks:**
- [ ] Implement OIDC authentication flow
- [ ] Create login page
- [ ] Implement session management
- [ ] Add token refresh
- [ ] Create user menu with logout

**Acceptance Criteria:**
- [ ] User can log in via OIDC
- [ ] Session persists across page loads
- [ ] Token refreshes before expiry
- [ ] User can log out

---

### Slice 9.5: Catalog Browser

**Files to create:**
```
solar-ui/src/
  app/
    catalog/
      page.tsx
      loading.tsx
      error.tsx
      [namespace]/
        [name]/
          page.tsx
  components/
    catalog/
      CatalogList.tsx
      CatalogCard.tsx
      CatalogDetail.tsx
      CatalogFilters.tsx
      VersionSelector.tsx
      DependencyGraph.tsx
```

**Tasks:**
- [ ] Implement catalog list view with search/filter
- [ ] Implement catalog item detail view
- [ ] Show version history and selector
- [ ] Display dependencies as graph
- [ ] Show attestation status
- [ ] Add accessibility features

**Acceptance Criteria:**
- [ ] Can browse catalog items
- [ ] Search and filters work
- [ ] Version selection works
- [ ] Dependencies are displayed
- [ ] Keyboard navigation works

---

### Slice 9.6: Cluster Management

**Files to create:**
```
solar-ui/src/
  app/
    clusters/
      page.tsx
      new/
        page.tsx
      [namespace]/
        [name]/
          page.tsx
  components/
    clusters/
      ClusterList.tsx
      ClusterCard.tsx
      ClusterRegistrationForm.tsx
      ClusterDetail.tsx
      AgentConfigDownload.tsx
      ClusterHealthIndicator.tsx
```

**Tasks:**
- [ ] Implement cluster list view
- [ ] Implement cluster registration form
- [ ] Generate and display agent configuration
- [ ] Show cluster health status
- [ ] List installed releases per cluster
- [ ] Show capacity usage

**Acceptance Criteria:**
- [ ] Can register new cluster
- [ ] Agent config downloads correctly
- [ ] Health status displayed
- [ ] Releases listed per cluster

---

### Slice 9.7: Release Management

**Files to create:**
```
solar-ui/src/
  app/
    releases/
      page.tsx
      new/
        page.tsx
      [namespace]/
        [name]/
          page.tsx
  components/
    releases/
      ReleaseList.tsx
      ReleaseForm.tsx
      ReleaseDetail.tsx
      ValuesEditor.tsx
      PreflightResults.tsx
      ReleaseTimeline.tsx
```

**Tasks:**
- [ ] Implement release list with status indicators
- [ ] Implement release creation wizard
- [ ] Add values editor (Monaco editor with YAML)
- [ ] Show preflight check results
- [ ] Show release history/timeline
- [ ] Add suspend/resume controls

**Acceptance Criteria:**
- [ ] Can create releases
- [ ] Values editor validates YAML
- [ ] Preflight results displayed
- [ ] Suspend/resume works

---

### Slice 9.8: Dashboard & Overview

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
      QuickActions.tsx
```

**Tasks:**
- [ ] Implement dashboard with key metrics
- [ ] Show recent activity feed
- [ ] Display cluster health overview
- [ ] Add quick action buttons
- [ ] Implement real-time updates

**Acceptance Criteria:**
- [ ] Dashboard shows aggregate data
- [ ] Activity updates in real-time
- [ ] Health grid accurate
- [ ] Quick actions work

---

### Slice 9.9: Sync Management (Catalog Chaining)

**Files to create:**
```
solar-ui/src/
  app/
    sync/
      page.tsx
      new/
        page.tsx
      [namespace]/
        [name]/
          page.tsx
  components/
    sync/
      SyncList.tsx
      SyncForm.tsx
      SyncDetail.tsx
      SyncStatus.tsx
```

**Tasks:**
- [ ] Implement sync list view
- [ ] Implement sync creation form
- [ ] Show sync status and history
- [ ] Display synced components

**Acceptance Criteria:**
- [ ] Can create syncs
- [ ] Status displayed correctly
- [ ] History shows sync events

---

### Slice 9.10: UI Testing & Observability

**Files to create:**
```
solar-ui/
  __tests__/
    components/
    pages/
  e2e/
    catalog.spec.ts
    clusters.spec.ts
    releases.spec.ts
```

**Tasks:**
- [ ] Set up Vitest for unit tests
- [ ] Set up Playwright for E2E tests
- [ ] Add client-side error tracking (Sentry optional)
- [ ] Implement performance monitoring
- [ ] Configure CSP headers

**Acceptance Criteria:**
- [ ] Unit tests pass (70%+ coverage)
- [ ] E2E tests for critical flows
- [ ] Errors are captured
- [ ] Performance metrics collected

---

## Phase 10: Integration, E2E Testing & Deployment

**Goal**: Validate complete system and prepare for production.

**Test Coverage Target**: 85%+ overall

### Slice 10.1: Integration Test Infrastructure

**Files to create:**
```
test/
  integration/
    suite_test.go
    setup.go
    helpers.go
    catalog_test.go
    cluster_test.go
    release_test.go
    discovery_test.go
    renderer_test.go
```

**Tasks:**
- [ ] Set up envtest for integration tests
- [ ] Create test fixtures for all resource types
- [ ] Implement helper functions for common operations
- [ ] Add CI job for integration tests
- [ ] Test all API operations

**Acceptance Criteria:**
- [ ] All resource types have integration tests
- [ ] Tests pass in CI
- [ ] Test data is isolated between tests

---

### Slice 10.2: End-to-End Test Suite

**Files to create:**
```
test/
  e2e/
    suite_test.go
    setup.go
    workflow_test.go
    chaining_test.go
    failure_test.go
```

**Tasks:**
- [ ] Set up kind cluster for E2E
- [ ] Deploy FluxCD to test cluster
- [ ] Implement full deployment workflow test
- [ ] Test catalog chaining scenario
- [ ] Test failure scenarios and recovery
- [ ] Add E2E job to CI

**E2E test scenarios:**
1. Full deployment workflow:
   - Scan registry → CatalogItem created
   - Register cluster → Agent connects
   - Create Release → Manifests rendered → FluxCD reconciles
   - Status reported back

2. Catalog chaining:
   - Create Sync → ARC Order created
   - Components synced to destination

3. Failure scenarios:
   - Registry unavailable
   - Agent disconnection
   - Preflight check failure

**Acceptance Criteria:**
- [ ] Full workflow test passes
- [ ] Chaining test passes
- [ ] Failure tests verify recovery
- [ ] E2E runs in CI

---

### Slice 10.3: Helm Charts

**Files to create:**
```
charts/
  solar/
    Chart.yaml
    values.yaml
    values.schema.json
    templates/
      _helpers.tpl
      solar-index/
        deployment.yaml
        service.yaml
        rbac.yaml
        configmap.yaml
      solar-discovery/
        deployment.yaml
        rbac.yaml
        configmap.yaml
      solar-renderer/
        deployment.yaml
        rbac.yaml
        configmap.yaml
      solar-ui/
        deployment.yaml
        service.yaml
        ingress.yaml
      crds/
        catalogitem.yaml
        clustercatalogitem.yaml
        clusterregistration.yaml
        release.yaml
        sync.yaml
  solar-agent/
    Chart.yaml
    values.yaml
    templates/
      deployment.yaml
      rbac.yaml
      configmap.yaml
      secret.yaml
```

**Tasks:**
- [ ] Create umbrella Helm chart for solar components
- [ ] Create separate chart for solar-agent
- [ ] Parameterize all configuration
- [ ] Add RBAC resources
- [ ] Add NetworkPolicies
- [ ] Add PodDisruptionBudgets
- [ ] Create values schema for validation
- [ ] Test installation in clean cluster

**Acceptance Criteria:**
- [ ] `helm install` succeeds
- [ ] All resources created correctly
- [ ] Configuration is parameterized
- [ ] NetworkPolicies limit traffic

---

### Slice 10.4: Production Hardening

**Tasks:**
- [ ] Security audit and fixes
  - Run gosec, trivy on all images
  - Review RBAC permissions (least privilege)
  - Audit authentication/authorization
- [ ] Performance testing and optimization
  - Load test with k6
  - Optimize hot paths
  - Add appropriate caching
- [ ] Add resource limits to all components
- [ ] Configure PodDisruptionBudgets
- [ ] Set up monitoring dashboards
- [ ] Configure alerting rules
- [ ] Document runbooks

**Acceptance Criteria:**
- [ ] Security scan passes (no critical/high)
- [ ] Load test meets SLOs
- [ ] All resources have limits
- [ ] Dashboards functional
- [ ] Alerts configured

---

### Slice 10.5: Documentation

**Files to create:**
```
docs/
  getting-started.md
  architecture.md
  api-reference.md
  deployment.md
  operations.md
  troubleshooting.md
  development.md
CONTRIBUTING.md
ARCHITECTURE.md
```

**Tasks:**
- [ ] Write getting started guide
- [ ] Document architecture decisions
- [ ] Generate API reference from OpenAPI
- [ ] Write deployment guide
- [ ] Write operations guide
- [ ] Add troubleshooting guide
- [ ] Write development guide

**Acceptance Criteria:**
- [ ] Documentation is complete
- [ ] Getting started works end-to-end
- [ ] API reference is accurate

---

## Testing Strategy

### Test Pyramid

```
                    /\
                   /  \
                  / E2E \        (10%)
                 /______\
                /        \
               /Integration\     (30%)
              /______________\
             /                \
            /    Unit Tests    \  (60%)
           /____________________\
```

### Coverage Targets by Phase

| Phase | Target | Rationale |
|-------|--------|-----------|
| 1 (Foundation) | 70% | Utility packages, less critical |
| 2 (API Types) | 75% | Type validation critical |
| 3-5 (solar-index) | 80% | Core API logic |
| 6-8 (Controllers) | 80% | Business logic |
| 9 (UI) | 70% unit, E2E critical | Visual components harder to unit test |
| 10 (Integration) | 85%+ overall | Production readiness |

### Testing Tools

| Tool | Purpose | Version |
|------|---------|---------|
| go test | Unit tests | Built-in |
| envtest | K8s integration | v0.22.x |
| kind | E2E clusters | v0.30.0 |
| Vitest | UI unit tests | Latest |
| Playwright | UI E2E | Latest |
| k6 | Load testing | Latest |

---

## Dependency Graph

```
Phase 1: Foundation
    │
    ▼
Phase 2: API Types
    │
    ├───────────────────────────────────┐
    ▼                                   │
Phase 3-5: solar-index                  │
    │                                   │
    ├──────────┬──────────┬────────────┤
    ▼          ▼          ▼            ▼
Phase 6    Phase 7    Phase 8     Phase 9
discovery  renderer    agent        UI
    │          │          │            │
    └──────────┴──────────┴────────────┘
                    │
                    ▼
            Phase 10: Integration
```

### Critical Path

1. Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5 (sequential)
2. Phases 6, 7, 8 can proceed in parallel after Phase 5
3. Phase 9 (UI) can start after Phase 2, but API integration requires Phase 5
4. Phase 10 requires all previous phases

---

## Risk Mitigation

### Technical Risks

| Risk | Mitigation |
|------|------------|
| Extension APIServer complexity | Start with apiserver-kit, follow k8s sample-apiserver patterns |
| OCM SDK changes | Pin to v0.31.0, monitor for breaking changes |
| FluxCD API changes | Use stable v2 APIs, avoid alpha features |
| Performance at scale | Early load testing, implement caching |

### Schedule Risks

| Risk | Mitigation |
|------|------------|
| Scope creep | Strict phase boundaries, defer non-critical features |
| Integration issues | Continuous integration testing from Phase 6 |
| Third-party dependencies | Pin versions, have fallback options |

---

## Success Criteria

### MVP (Phases 1-8)

- [ ] solar-index serves all APIs correctly
- [ ] solar-discovery scans registries and creates CatalogItems
- [ ] solar-renderer renders manifests and publishes OCI images
- [ ] solar-agent deploys via FluxCD and reports status
- [ ] 80%+ test coverage for backend
- [ ] All security scans pass

### Production Ready (Phase 10)

- [ ] Full E2E workflow functional
- [ ] 85%+ overall test coverage
- [ ] Helm charts validated
- [ ] Documentation complete
- [ ] Performance meets SLOs
- [ ] Security audit passed

---

## References

- [Kubernetes Extension APIServer](https://github.com/kubernetes/apiserver)
- [apiserver-kit](https://github.com/opendefensecloud/apiserver-kit)
- [Open Component Model](https://ocm.software/)
- [FluxCD](https://fluxcd.io/)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [Next.js](https://nextjs.org/)
- [shadcn/ui](https://ui.shadcn.com/)
- [OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/)
