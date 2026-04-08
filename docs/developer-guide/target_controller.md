# Target Controller Documentation

## Overview

The Target controller manages the lifecycle of `Target` custom resources in SolAr. It creates and manages a `Bootstrap` resource that combines the Target's releases with matching Profiles based on label selectors.

## Architecture

```mermaid
flowchart TD
    subgraph Kubernetes
        Ctrl[Target Controller]
        T[Target]
        P[Profile]
        BS[Bootstrap]
    end

    Ctrl -->|reconciles| T
    Ctrl -->|watches| P

    T -->|creates/updates| BS
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
            C->>K: Get Bootstrap
            alt Bootstrap not found
                C->>K: Create Bootstrap
            else Bootstrap out of sync
                C->>K: Update Bootstrap spec
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
        BS[Bootstrap]
    end

    T -->|owns| BS
```

| Resource       | Name Pattern             | Namespace  |
| -------------- | --------------          | ----------- |
| Bootstrap | `<target-name>`         | Inherited  |

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
- The matched Profiles are stored in the Bootstrap's `spec.profiles` field

## Cleanup Behavior

- **On Target deletion**: Deletes the associated Bootstrap, then removes finalizer
- **On Profile `targetSelector` change**: Updates all affected Bootstraps to reflect new profile matches

## Controller Configuration

Configuration of the controller is managed by the controller manager. The Target controller can be configured with the following parameters:

| Parameter        | Type        | Description                                        |
| ---              | ---         | ---                                                |
| `WatchNamespace` | `string`    | (Test only) Restrict reconciliation to this namespace |
