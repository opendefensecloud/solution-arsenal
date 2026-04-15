# Target Controller Documentation

## Overview

The Target controller is the central orchestrator of the SolAr rendering pipeline. It manages the lifecycle of `Target` custom resources and drives the two-stage rendering pipeline that produces deployable Helm charts for each target cluster.

For each Target, the controller:

1. Resolves the render `Registry` referenced by `spec.renderRegistryRef`.
2. Collects all `ReleaseBinding` resources that reference the Target.
3. Creates a per-release `RenderTask` for each bound Release (Stage 1).
4. Once all release RenderTasks succeed, creates a bootstrap `RenderTask` that bundles all rendered release charts (Stage 2).
5. Manages cleanup of stale RenderTasks when the release set changes.

See [Rendering Pipeline](./rendering-pipeline.md) for a detailed description of the two-stage pipeline.

## Architecture

```mermaid
flowchart TD
    subgraph Inputs
        RB[ReleaseBindings]
        Reg[Registry]
        Rel[Releases]
    end

    subgraph Target Controller
        Ctrl[TargetReconciler]
    end

    subgraph Outputs
        RT1["RenderTask\n(per release)"]
        RT2["RenderTask\n(bootstrap)"]
    end

    RB -->|triggers| Ctrl
    Reg -->|triggers| Ctrl
    Rel -->|triggers| Ctrl
    RT1 -->|status change triggers| Ctrl
    RT2 -->|status change triggers| Ctrl

    Ctrl -->|creates/tracks| RT1
    Ctrl -->|creates/tracks| RT2
```

## Status Conditions

```mermaid
stateDiagram-v2
    [*] --> RegistryUnresolved: Target created
    RegistryUnresolved --> RegistryResolved: Registry found with SolarSecretRef
    RegistryResolved --> ReleasesRendering: ReleaseBindings exist
    ReleasesRendering --> ReleasesRendered: All release RenderTasks succeeded
    ReleasesRendered --> BootstrapRendering: Bootstrap RenderTask created
    BootstrapRendering --> BootstrapReady: Bootstrap RenderTask succeeded
    BootstrapRendering --> BootstrapFailed: Bootstrap RenderTask failed
    ReleasesRendering --> ReleasesFailed: Any release RenderTask failed
```

| Condition            | Status  | Reason                  | Description                                          |
| -------------------- | ------- | ----------------------- | ---------------------------------------------------- |
| `RegistryResolved`   | `True`  | `Resolved`              | Registry found and has `solarSecretRef`              |
| `RegistryResolved`   | `False` | `NotFound`              | Registry resource not found                          |
| `RegistryResolved`   | `False` | `MissingSolarSecretRef` | Registry exists but lacks push credentials           |
| `ReleasesRendered`   | `True`  | `AllRendered`           | All release RenderTasks completed successfully       |
| `ReleasesRendered`   | `False` | `NoBindings`            | No ReleaseBindings found for this Target             |
| `ReleasesRendered`   | `False` | `Pending`               | Waiting for release RenderTasks to complete          |
| `ReleasesRendered`   | `False` | `MissingDependencies`   | One or more Releases or ComponentVersions not found  |
| `ReleasesRendered`   | `False` | `ReleaseFailed`         | At least one release RenderTask failed               |
| `BootstrapReady`     | `True`  | `Ready`                 | Bootstrap RenderTask succeeded; `ChartURL` populated |
| `BootstrapReady`     | `False` | `Failed`                | Bootstrap RenderTask failed                          |

## Finalizer

The Target controller adds the finalizer `solar.opendefense.cloud/target-finalizer` to every Target. On deletion, it:

1. Deletes all RenderTasks owned by the Target.
2. Removes the finalizer to allow the Target to be garbage-collected.

## RenderTask Naming

| RenderTask type | Name pattern                            |
| --------------- | --------------------------------------- |
| Release         | `render-rel-<release-name>-<hash>`      |
| Bootstrap       | `render-tgt-<target-name>-<version>`    |

## Bootstrap Versioning

The bootstrap chart version is incremented whenever the set of bound releases or their resolved content changes, ensuring a new chart is pushed whenever the desired state changes. Stale RenderTasks from prior versions are cleaned up after the current bootstrap succeeds.

## Watch Triggers

| Watched Resource  | Mapping                                           |
| ----------------- | ------------------------------------------------- |
| `Target`          | Direct reconcile of the Target                    |
| `ReleaseBinding`  | Reconcile the Target referenced by the binding    |
| `RenderTask`      | Reconcile the owning Target (status change only)  |
| `Registry`        | Reconcile all Targets that reference the Registry |
| `Release`         | Reconcile all Targets bound to the Release        |

## Sequence Diagrams

### New Release added via Profile (triggers bootstrap re-render)

```mermaid
sequenceDiagram
    participant User as Deployment Coordinator
    participant K8s as Kubernetes API
    participant ProfileCtrl as Profile Controller
    participant TargetCtrl as Target Controller
    participant RenderTaskCtrl as RenderTask Controller
    participant Registry as OCI Registry

    User->>K8s: Create Profile (targetSelector, releaseRef)
    K8s->>ProfileCtrl: Reconcile(Profile)
    ProfileCtrl->>K8s: Create ReleaseBinding (Target ← Release)

    K8s->>TargetCtrl: Reconcile(Target) [new ReleaseBinding]
    TargetCtrl->>K8s: Create release RenderTask
    K8s->>RenderTaskCtrl: Reconcile(RenderTask)
    RenderTaskCtrl->>Registry: Push release chart
    RenderTaskCtrl->>K8s: Update RenderTask status (succeeded)

    K8s->>TargetCtrl: Reconcile(Target) [RenderTask status changed]
    TargetCtrl->>K8s: Update status (ReleasesRendered=True)
    Note over TargetCtrl: Bootstrap input changed → create new bootstrap RenderTask,<br/>delete stale one from prior version
    TargetCtrl->>K8s: Create bootstrap RenderTask
    K8s->>RenderTaskCtrl: Reconcile(RenderTask)
    RenderTaskCtrl->>Registry: Push bootstrap chart
    RenderTaskCtrl->>K8s: Update RenderTask status (succeeded)

    K8s->>TargetCtrl: Reconcile(Target) [RenderTask status changed]
    TargetCtrl->>K8s: Update status (BootstrapReady=True)
```

### Target deletion

```mermaid
sequenceDiagram
    participant User as Cluster Maintainer
    participant K8s as Kubernetes API
    participant TargetCtrl as Target Controller

    User->>K8s: Delete Target
    K8s->>TargetCtrl: Reconcile(Target) [DeletionTimestamp set]
    TargetCtrl->>K8s: List owned RenderTasks
    TargetCtrl->>K8s: Delete all owned RenderTasks
    TargetCtrl->>K8s: Remove finalizer
    Note over K8s: Target is garbage-collected
```

## Relationship to Other Controllers

```mermaid
flowchart LR
    ProfileCtrl[Profile Controller] -->|creates| ReleaseBinding
    ReleaseBinding -->|triggers| TargetCtrl[Target Controller]
    TargetCtrl -->|creates| RenderTask
    RenderTask -->|managed by| RenderTaskCtrl[RenderTask Controller]
    RenderTaskCtrl -->|status update triggers| TargetCtrl
```

