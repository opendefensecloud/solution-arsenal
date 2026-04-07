# Target Controller Documentation

## Overview

The Target controller manages the lifecycle of `Target` custom resources in SolAr. It creates and manages a `HydratedTarget` resource that combines the Target's releases with matching Profiles based on label selectors.

## Architecture

```mermaid
flowchart TD
    subgraph Kubernetes
        Ctrl[Target Controller]
        T[Target]
        P[Profile]
        HT[HydratedTarget]
    end

    Ctrl -->|reconciles| T
    Ctrl -->|watches| P

    T -->|creates/updates| HT
    T -->|matches labels| P
```

## Reconcile Loop

```mermaid
sequenceDiagram
    actor U as User / GitOps
    participant K as Kubernetes API
    participant C as target-controller
    participant P as Profile

    U->>K: Create Target
    K-->>U: Target created

    loop Reconcile Loop
        K->>C: Watch Event (Target)
        C->>K: Get Target
        alt Target found
            C->>K: List Profiles (namespace)
            C->>K: Match Profiles by label selector
            C->>K: Get HydratedTarget
            alt HydratedTarget not found
                C->>K: Create HydratedTarget
            else HydratedTarget out of sync
                C->>K: Update HydratedTarget spec
            end
        else Not found
            C-->>C: No-op / requeue
        end
    end

    P->>K: Update Profile (targetSelector)
    K->>C: Watch Event (Profile)
    C->>K: List matching Targets
    C->>K: Enqueue reconcile for each Target
```

## Resource Owner References

```mermaid
flowchart LR
    subgraph Target
        T[Target]
    end

    subgraph Owned Resources
        HT[HydratedTarget]
    end

    T -->|owns| HT
```

| Resource       | Name Pattern             | Namespace  |
| -------------- | --------------          | ----------- |
| HydratedTarget | `<target-name>`         | Inherited  |

## Profile Matching Logic

The controller matches Targets to Profiles using Kubernetes label selectors:

```mermaid
flowchart LR
    subgraph Target
        TL[Labels]
    end

    subgraph Profile
        PS[targetSelector]
    end

    TL -->|matches| PS
```

- A Profile's `targetSelector` field defines a label selector
- When a Profile is created or updated, all matching Targets trigger reconciliation
- The matched Profiles are stored in the HydratedTarget's `spec.profiles` field

## Cleanup Behavior

- **On Target deletion**: Deletes the associated HydratedTarget, then removes finalizer
- **On Profile change**: Updates all affected HydratedTargets to reflect new profile matches

## Controller Configuration

Configuration of the controller is managed by the controller manager. The Target controller can be configured with the following parameters:

| Parameter        | Type        | Description                                        |
| ---              | ---         | ---                                                |
| `WatchNamespace` | `string`    | (Test only) Restrict reconciliation to this namespace |
