# HydratedTarget Controller Documentation

## Overview

The HydratedTarget controller manages the lifecycle of `HydratedTarget` custom resources in SolAr. It creates and manages a `RenderTask` that triggers the rendering of a composite Helm chart combining multiple Release charts.

## Architecture

```mermaid
flowchart TD
    subgraph Kubernetes
        Ctrl[HydratedTarget Controller]
        HT[HydratedTarget]
        Rel[Release]
        RT[RenderTask]
    end

    subgraph Registry
        Chart[OCI Helm Chart]
    end

    Ctrl -->|reconciles| HT

    HT -->|references| Rel
    HT -->|waits for| Rel
    HT -->|creates/updates| RT

    RT -->|creates| Job[Render Job]
    Job -->|pushes| Chart
```

## Reconcile Loop

```mermaid
sequenceDiagram
    actor U as User / GitOps
    participant K as Kubernetes API
    participant C as hydratedtarget-controller
    participant Rel as Release

    Note over U,C: (HydratedTarget usually created by Target controller)

    loop Reconcile Loop
        K->>C: Watch Event (HydratedTarget)
        C->>K: Get HydratedTarget
        C->>K: Get all Referenced Releases
        alt All Releases rendered successfully
            C->>K: Get RenderTask status
            alt RenderTask not found
                C->>K: Create RenderTask
            else RenderTask exists
                C->>K: Get RenderTask status
            end
            alt RenderTask status changed
                C->>K: Update HydratedTarget status
            end
        else Release not rendered yet
            C-->>C: Requeue after 30s
        else Release failed
            C-->>C: Log error, requeue after 30s
        end
    end

    Rel->>K: Release status changed
    K->>C: Watch Event (Release)
    C->>K: Reconcile HydratedTarget
```

## Resource Owner References

```mermaid
flowchart LR
    subgraph HydratedTarget
        HT[HydratedTarget]
    end

    subgraph Owned Resources
        RT[RenderTask]
    end

    HT -->|owns| RT
```

| Resource   | Name Pattern                               | Namespace  |
| ---------- | --------------                             | ----------- |
| RenderTask | `ht-<hydratedtarget-name>-<generation>`     | Inherited  |

## Dependency Chain

The HydratedTarget waits for all referenced Releases to be successfully rendered before creating its own RenderTask:

```mermaid
flowchart LR
    subgraph Dependencies
        Rel1[Release 1] -->|chartURL| HT[HydratedTarget]
        Rel2[Release 2] -->|chartURL| HT
        RelN[Release N] -->|chartURL| HT
    end

    HT -->|creates RenderTask| RT
```

This creates a dependency chain:
1. ComponentVersion discovered → Release can render
2. Release rendered → HydratedTarget can render

## Status Conditions

The controller updates the HydratedTarget status with the following conditions:

```mermaid
stateDiagram-v2
    [*] --> Initial: HydratedTarget Created
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

The HydratedTarget status also tracks:
- `ChartURL`: The URL of the rendered composite Helm chart in the OCI registry
- `RenderTaskRef`: Reference to the created RenderTask

## Cleanup Behavior

- **On deletion**: Deletes the associated RenderTask (with background propagation), then removes finalizer
- **On successful render**: HydratedTarget remains as-is (immutable once succeeded)
- **On failed render**: HydratedTarget remains with failed status; new RenderTask created on next spec change (new generation)

## Controller Configuration

Configuration of the controller is managed by the controller manager. The HydratedTarget controller can be configured with the following parameters:

| Parameter        | Type        | Description                                        |
| ---              | ---         | ---                                                |
| `WatchNamespace` | `string`    | (Test only) Restrict reconciliation to this namespace |
