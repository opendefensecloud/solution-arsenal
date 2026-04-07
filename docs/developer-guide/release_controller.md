# Release Controller Documentation

## Overview

The Release controller manages the lifecycle of `Release` custom resources in SolAr. It creates and manages a `RenderTask` that triggers the rendering of a Helm chart from a ComponentVersion.

## Architecture

```mermaid
flowchart TD
    subgraph Kubernetes
        Ctrl[Release Controller]
        Rel[Release]
        RT[RenderTask]
        CV[ComponentVersion]
    end

    subgraph Registry
        Chart[OCI Helm Chart]
    end

    Ctrl -->|reconciles| Rel

    Rel -->|references| CV
    Rel -->|creates/updates| RT

    RT -->|creates| Job[Render Job]
    Job -->|pushes| Chart
```

## Reconcile Loop

```mermaid
sequenceDiagram
    actor U as User / GitOps
    participant K as Kubernetes API
    participant C as release-controller
    participant CV as ComponentVersion
    participant RT as RenderTask

    U->>K: Create Release
    K-->>U: Release created

    loop Reconcile Loop
        K->>C: Watch Event (Release)
        C->>K: Get Release
        C->>K: Get ComponentVersion
        alt Release already succeeded
            C-->>C: No-op
        else Release already failed
            C-->>C: No-op
        else RenderTask not found
            C->>K: Create RenderTask
            K-->>C: RenderTask created
        else RenderTask exists
            C->>K: Get RenderTask status
        end
        alt RenderTask status changed
            C->>K: Update Release status
        end
    end

    loop RenderTask Lifecycle
        RT->>K: Create Render Job
        K-->>RT: Job created
        RT->>Chart: Push rendered chart
    end
```

## Resource Owner References

```mermaid
flowchart LR
    subgraph Release
        Rel[Release]
    end

    subgraph Owned Resources
        RT[RenderTask]
    end

    Rel -->|owns| RT
```

| Resource   | Name Pattern                          | Namespace  |
| ---------- | --------------                        | ----------- |
| RenderTask | `release-<release-name>-<generation>` | Inherited  |

## Status Conditions

The controller updates the Release status with the following conditions:

```mermaid
stateDiagram-v2
    [*] --> Initial: Release Created
    Initial --> RenderTaskCreated: RenderTask Created
    RenderTaskCreated --> TaskCompleted: RenderTask succeeded
    RenderTaskCreated --> TaskFailed: RenderTask failed
    RenderTaskCreated --> RenderTaskCreated: RenderTask running
    TaskCompleted --> [*]
    TaskFailed --> [*]
```

| Condition           | Status   | Reason       | Description                     |
| -----------         | -------- | --------     | ------------------------------- |
| `TaskCompleted`     | `True`   | TaskCompleted| RenderTask completed successfully|
| `TaskFailed`        | `True`   | TaskFailed   | RenderTask failed               |

The Release status also tracks:
- `ChartURL`: The URL of the rendered Helm chart in the OCI registry
- `RenderTaskRef`: Reference to the created RenderTask

## Cleanup Behavior

- **On deletion**: Deletes the associated RenderTask (with background propagation), then removes finalizer
- **On successful render**: Release remains as-is (immutable once succeeded)
- **On failed render**: Release remains with failed status; new RenderTask created on next spec change (new generation)

## Controller Configuration

Configuration of the controller is managed by the controller manager. The Release controller can be configured with the following parameters:

| Parameter        | Type        | Description                                        |
| ---              | ---         | ---                                                |
| `WatchNamespace` | `string`    | (Test only) Restrict reconciliation to this namespace |
