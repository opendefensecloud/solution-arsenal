# Discovery Controller Documentation

## Overview

The Discovery controller manages the lifecycle of `Discovery` custom resources in SolAr. It creates and manages a Pod that executes the discovery worker, along with associated Kubernetes resources for RBAC, networking, and configuration.

## Architecture

```mermaid
flowchart TD
    subgraph Kubernetes
        Ctrl[Discovery Controller]
        D[Discovery]
        SA[ServiceAccount]
        Role[Role]
        RB[RoleBinding]
        Secret[Config Secret]
        Svc[Service]
        Pod[Worker Pod]
    end

    subgraph Registry
        Reg[OCI Registry]
    end

    Ctrl -->|reconciles| D

    D -->|creates| SA
    D -->|creates| Role
    D -->|creates| RB
    D -->|creates| Secret
    D -->|creates| Svc
    D -->|creates| Pod

    Pod -->|scans/webhook| Reg
    Pod -->|writes| Comp[Component / ComponentVersion]
```

## Reconcile Loop

```mermaid
sequenceDiagram
    actor U as User / GitOps
    participant K as Kubernetes API
    participant C as discovery-controller
    participant W as discovery-worker

    U->>K: Create Discovery (namespace)
    K-->>U: Discovery created

    loop Reconcile Loop
        K->>C: Watch Event (Discovery)
        C->>K: Get Discovery (namespace)
        alt Discovery found
            C->>K: Create ServiceAccount
            C->>K: Create Role
            C->>K: Create RoleBinding
            C->>K: Create Config Secret
            C->>K: Create Service
            C->>K: Create Worker Pod
            K-->>C: Resources created
            K-->>W: Schedule & Start Pod
        else Not found
            C-->>C: No-op / requeue
        end
    end

    loop Worker Pipeline
        W->>Reg: Scan Registry / Receive Webhook Event
        Reg-->>W: Repository Events
        W->>K: Create Component / ComponentVersion
    end
```

## Resource Owner References

```mermaid
flowchart LR
    subgraph Discovery
        D[Discovery]
    end

    subgraph Owned Resources
        SA[ServiceAccount]
        Role[Role]
        RB[RoleBinding]
        Secret[Config Secret]
        Svc[Service]
        Pod[Pod]
    end

    D -->|owns| SA
    D -->|owns| Role
    D -->|owns| RB
    D -->|owns| Secret
    D -->|owns| Svc
    D -->|owns| Pod
```

| Resource        | Name Pattern                    | Namespace  |
| --------------- | --------------                  | ----------- |
| ServiceAccount  | `discovery-<discovery-name>`   | Inherited  |
| Role            | `solar-discovery-worker`        | Inherited  |
| RoleBinding     | `solar-discovery-worker`        | Inherited  |
| Config Secret   | `discovery-<discovery-name>`    | Inherited  |
| Service         | `discovery-<discovery-name>`    | Inherited  |
| Pod             | `discovery-<discovery-name>`    | Inherited  |

## Status Conditions

The controller updates the Discovery status with the following fields:

| Field           | Description                                          |
| --------------- | --------------------------------------------------- |
| `PodGeneration` | Tracks the generation of the Discovery spec for pod rollout detection |

## Cleanup Behavior

- **On deletion**: Deletes all owned resources (Pod, Service, Secret, ServiceAccount, Role, RoleBinding), then removes finalizer
- **On spec change**: Deletes and recreates all worker resources to ensure consistency

## Controller Configuration

Configuration of the controller is managed by the controller manager. The Discovery controller can be configured with the following parameters:

| Parameter       | Type        | Description                           |
| ---             | ---         | ---                                   |
| `WorkerImage`   | `string`    | Image to be used for the worker Pod    |
| `WorkerCommand` | `string`    | Command for the worker Pod             |
| `WorkerArgs`    | `[]string`  | Additional args for the worker Pod     |
| `WatchNamespace`| `string`    | (Test only) Restrict reconciliation to this namespace |
