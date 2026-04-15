# Architecture

SolAr is implemented as a Kubernetes Extension API Server integrated with the Kubernetes API Aggregation Layer. This architectural approach provides several advantages over Custom Resource Definitions (CRDs), including dedicated storage isolation, custom API implementation flexibility, and reduced risk to the hosting cluster's control plane.

```mermaid
graph TB
    subgraph "User Interface Layer"
        User["User/Operator"]
        Kubectl["kubectl CLI"]
        GitOps["GitOps Tools"]
    end

    subgraph "Kubernetes Control Plane"
        K8sAPI["Kubernetes API Server"]
        APIAgg["API Aggregation Layer"]

        subgraph "SolAr API Server"
            SOLARAPI["SolAr Extension API Server"]
            SOLARETCD["SolAr etcd<br/>Isolated Storage"]
        end
    end

    subgraph "SolAr Controller Manager"
        TargetCtrl["Target Controller"]
        ReleaseCtrl["Release Controller"]
        ProfileCtrl["Profile Controller"]
        RenderTaskCtrl["RenderTask Controller"]
    end

    subgraph "SolAr Discovery (standalone)"
        Discovery["solar-discovery<br/>Scans OCI registries<br/>for OCM packages"]
    end

    subgraph "External Systems"
        SrcReg["Source Systems<br/>OCI Registries, S3,<br/>Helm Repos, HTTP"]
        DstReg["Render Registries<br/>OCI Registries for<br/>rendered charts"]
    end

    User -->|"Creates Releases,<br/>Targets, Profiles"| Kubectl
    GitOps -->|"Declarative Config"| Kubectl
    Kubectl -->|"API Requests"| K8sAPI

    K8sAPI <-->|"Routes solar.opendefense.cloud"| APIAgg
    APIAgg <-->|"Custom Resources"| SOLARAPI
    SOLARAPI <-->|"Persists"| SOLARETCD
```

**Architecture: SolAr System Components and Data Flow**

The system follows a layered architecture where users interact through `kubectl` (or GitOps tools), requests flow through the Kubernetes API aggregation layer to the SolAr API Server. Controllers reconcile the declared resources and drive the rendering pipeline.

**Key Design Decisions:**

- **Extension API Server architecture** provides dedicated storage isolation in a separate etcd instance
- **Declarative, Kubernetes-native API** for GitOps compatibility

## Resource Model and Dependencies

```mermaid
graph TB
    subgraph "Catalog Resources"
        Component["Component"]
        ComponentVersion["ComponentVersion"]
    end

    subgraph "Deployment Resources"
        Release["Release"]
        Target["Target"]
        Profile["Profile"]
    end

    subgraph "Binding Resources"
        ReleaseBinding["ReleaseBinding"]
        Registry["Registry"]
    end

    subgraph "Internal Resources"
        RenderTask["RenderTask"]
    end

    SolArDiscovery["solar-discovery<br/>(standalone)"] -->|"discovers"| ComponentVersion
    SolArDiscovery -->|"discovers"| Component

    ComponentVersion -->|"references"| Component
    Release -->|"references"| ComponentVersion

    Profile -->|"references"| Release
    Profile -->|"creates"| ReleaseBinding

    ReleaseBinding -->|"binds"| Release
    ReleaseBinding -->|"binds"| Target

    Target -->|"references"| Registry
    Target -->|"creates"| RenderTask

    Registry -->|"provides credentials<br/>and hostname"| RenderTask
```

### Resource Roles

- **Component / ComponentVersion** — catalog entries discovered from OCI registries by solar-discovery.
- **Release** — declares which ComponentVersion to deploy and with what configuration.
- **Target** — represents a deployment target (cluster). References a Registry via `renderRegistryRef` for pushing rendered charts.
- **Registry** — stores OCI registry hostname and push credentials (`solarSecretRef`).
- **ReleaseBinding** — declares that a Release should be deployed to a Target. Created manually or automatically by the Profile controller.
- **Profile** — matches Targets by label selector and automatically creates ReleaseBindings for a given Release.
- **RenderTask** — internal resource created by the Target controller to drive chart rendering jobs.

## Controllers

- [Rendering pipeline](./rendering-pipeline.md) — how Targets, Releases, and RenderTasks produce deployable Helm charts
- [Release controller](./release_controller.md) — validates Release → ComponentVersion references
- [Profile controller](./profile_controller.md) — automates ReleaseBinding creation via label selectors
- [Target controller](./target_controller.md) — orchestrates the rendering pipeline per target cluster
- [RenderTask controller](./rendertask_controller.md) — lifecycle of individual RenderTask resources

## Discovery

- [Discovery pipeline](./discovery_pipeline.md) — how solar-discovery scans OCI registries and writes ComponentVersions
