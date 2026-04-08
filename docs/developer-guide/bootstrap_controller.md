# Bootstrap Controller Documentation

## Overview

The Bootstrap controller manages the lifecycle of `Bootstrap` custom resources in SolAr. It creates and manages a `RenderTask` that triggers the rendering of a composite Helm chart combining multiple Release charts.

## Architecture

```mermaid
flowchart TD
    subgraph Kubernetes
        Ctrl[Bootstrap Controller]
        BS[Bootstrap]
        Rel[Release]
        RT[RenderTask]
    end

    subgraph Registry
        Chart[OCI Helm Chart]
    end

    Ctrl -->|reconciles| BS

    BS -->|references| Rel
    BS -->|waits for| Rel
    BS -->|creates/updates| RT

    RT -->|creates| Job[Render Job]
    Job -->|pushes| Chart
```

## Reconcile Loop

```mermaid
sequenceDiagram
    actor U as User / GitOps
    participant K as Kubernetes API
    participant C as bootstrap-controller
    participant Rel as Release

    Note over U,C: (Bootstrap usually created by Target controller)

    loop Reconcile Loop
        K->>C: Watch Event (Bootstrap)
        C->>K: Get Bootstrap
        C->>K: Get all Referenced Releases and Profiles
        alt All Releases rendered successfully
            C->>K: Get RenderTask status
            alt RenderTask not found
                C->>K: Create RenderTask
            else RenderTask exists
                C->>K: Get RenderTask status
            end
            alt RenderTask status changed
                C->>K: Update Bootstrap status
            end
        else Release not rendered yet
            C-->>C: Requeue after 30s
        else Release failed
            C-->>C: Log error, requeue after 30s
        end
    end

    Note over RT,C: (RenderTask status changes trigger Bootstrap reconciliation)
    RT->>K: RenderTask status changed
    K->>C: Watch Event (RenderTask)
    C->>K: Reconcile Bootstrap
```

## Resource Owner References

```mermaid
flowchart LR
    subgraph Bootstrap
        BS[Bootstrap]
    end

    subgraph Owned Resources
        RT[RenderTask]
    end

    BS -->|owns| RT
```

| Resource   | Name Pattern                                  | Namespace  |
| ---------- | --------------                              | ----------- |
| RenderTask | `<namespace>-<bootstrap-name>-<generation>` (e.g., `testns-test-bs-0`) | Inherited  |

## Dependency Chain

The Bootstrap waits for all referenced Releases to be successfully rendered before creating its own RenderTask. Releases can be referenced directly or indirectly via Profiles:

```mermaid
flowchart LR
    subgraph Dependencies
        Rel1[Release 1] -->|chartURL| BS[Bootstrap]
        Rel2[Release 2] -->|chartURL| BS
        P[Profile] -->|resolves to Release| BS
    end

    BS -->|creates RenderTask| RT
```

This creates a dependency chain:
1. ComponentVersion discovered → Release can render
2. All Releases rendered (direct + via Profiles) → Bootstrap can render

## Status Conditions

The controller updates the Bootstrap status with the following conditions:

```mermaid
stateDiagram-v2
    [*] --> Initial: Bootstrap Created
    Initial --> WaitingForReleases: Checking Release status
    WaitingForReleases --> RenderTaskCreated: All Releases rendered
    WaitingForReleases --> WaitingForReleases: Releases pending
    WaitingForReleases --> Error: Release failed
    RenderTaskCreated --> TaskCompleted: RenderTask succeeded
    RenderTaskCreated --> TaskFailed: RenderTask failed
    RenderTaskCreated --> RenderTaskCreated: RenderTask running
    TaskCompleted --> [*]
    TaskFailed --> [*]
```

| Condition           | Status   | Reason       | Description                      |
| -----------         | -------- | --------     | -------------------------------- |
| `TaskCompleted`     | `True`   | TaskCompleted| RenderTask completed successfully |
| `TaskFailed`        | `True`   | TaskFailed   | RenderTask failed                |

The Bootstrap status also tracks:
- `RenderTaskRef`: Reference to the created RenderTask

## Cleanup Behavior

- **On deletion**: Deletes the associated RenderTask (with background propagation), then removes finalizer
- **On successful render**: Bootstrap remains as-is (immutable once succeeded)
- **On failed render**: Bootstrap remains with failed status; new RenderTask created on next spec change (new generation)

## Controller Configuration

Configuration of the controller is managed by the controller manager. The Bootstrap controller can be configured with the following parameters:

| Parameter        | Type        | Description                                        |
| ---              | ---         | ---                                                |
| `WatchNamespace` | `string`    | (Test only) Restrict reconciliation to this namespace |
