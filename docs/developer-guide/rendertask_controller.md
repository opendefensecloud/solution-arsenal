# RenderTask Controller Documentation

## Overview

The RenderTask controller manages the lifecycle of `RenderTask` custom
resources in SolAr. It creates and manages a Kubernetes Job that executes the
renderer container, along with associated Secrets for configuration and
authentication.

A RenderTask is immutable once created.

## Architecture

```mermaid
flowchart TD
    subgraph Kubernetes
        Ctrl[RenderTask Controller]
        RT[RenderTask]
        J[Job]
        J -->|creates| Renderer[Renderer Pod]
        CS[Config Secret]
        AS[Auth Secret]
    end

    subgraph Registry
        Chart[OCI Helm Chart]
    end

    Ctrl -->|reconciles| RT

    RT -->|creates| CS
    RT -->|creates| AS
    RT -->|creates| J

    Renderer -->|pushes| Chart

    Renderer -.-|mounts| CS
    Renderer -.-|mounts| AS
```

## Reconcile Loop

```mermaid
sequenceDiagram
    actor K as Kubernetes API
    participant C as rendertask-controller
    participant J as Render Job
    participant R as Registry

    Note over C: (Triggered by Release or HydratedTarget controller)

    loop Reconcile Loop
        K->>C: Watch Event (RenderTask)
        C->>K: Get RenderTask
        alt RenderTask not found
            C-->>C: No-op
        else JobScheduled or JobSucceeded
            C-->>C: No-op
        else Job not found
            C->>K: Create Config Secret
            C->>K: Create Render Job
            K-->>J: Job created
        else Job running
            C->>K: Get Job status
        end
        alt Job status changed
            C->>K: Update RenderTask status
        end
    end

    loop Job Lifecycle
        J->>R: Push Helm Chart
        alt Job succeeded
            J->>K: Update Job status
            C->>K: Delete Config Secret
            C->>K: Delete Job (TTL)
        else Job failed
            J->>K: Update Job status
            C->>K: Wait for TTL
            C->>K: Delete Config Secret
        end
    end
```

## Resource Owner References

```mermaid
flowchart LR
    subgraph RenderTask
        RT[RenderTask]
    end

    subgraph Owned Resources
        JS[Job]
        CS[Config Secret]
        AS[Auth Secret]
    end

    RT -->|owns| JS
    RT -->|owns| CS
    RT -->|owns| AS
```

## Status Conditions

The controller updates the RenderTask status with the following conditions:

```mermaid
stateDiagram-v2
    [*] --> JobScheduled: Job Created
    JobScheduled --> JobSucceeded: job.Status.Succeeded > 0
    JobScheduled --> JobFailed: job.Status.Failed > 0
    JobScheduled --> JobScheduled: job active
    JobSucceeded --> [*]
    JobFailed --> [*]
```

| Condition      | Status   | Reason                     |
| -----------    | -------- | --------                   |
| `JobScheduled` | `True`   | Job is running (active)    |
| `JobScheduled` | `False`  | Job does not exist         |
| `JobSucceeded` | `True`   | Job completed successfully |
| `JobFailed`    | `True`   | Job failed                 |

## Resource Naming Convention

| Resource     | Name Pattern               | Namespace   |
| ----------   | --------------             | ----------- |
| RenderJob    | `render-<rendertask-name>` | Inherited   |
| ConfigSecret | `render-<rendertask-name>` | Inherited   |
| AuthSecret   | `auth-<rendertask-name>`   | Inherited   |

## Cleanup Behavior

- **On successful completion**: Deletes Job, config Secret, and auth Secret
- **On deletion**: Deletes Job, config Secret, and auth Secret, then removes finalizer
- **TTL**: Job has `TTLSecondsAfterFinished: 3600` (1 hour) as fallback cleanup

## Controller Configuration

Configuration of the controller is managed by the controller manager. The
RenderTask controller can be configured with the following parameters:

| Parameter         | Type                      | Description
| ---               | ---                       | ---
| `RendererImage`   | `string`                  | Image to be used for the render Job / Pod
| `RendererCommand` | `string`                  | Command for the render Job / Pod
| `RendererArgs`    | `[]string`                | Additional args for the render Job / Pod
| `BaseURL`         | `string`                  | URL of the registry to which rendered charts get pushed to
| `PushSecretRef`   | `*corev1.SecretReference` | (Optional) Reference to a secret containing credentials for the registry

If PushSecretRef is set, the controller copies the secret to the Job's
Namespace so it can be mounted by the Pod. The secret gets cleaned up together
with the other RenderTask Resources.
