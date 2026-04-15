# Profile Controller Documentation

## Overview

The Profile controller manages the lifecycle of `Profile` custom resources in SolAr. It evaluates each Profile's `targetSelector` label selector against all Targets in the same namespace and creates or deletes `ReleaseBinding` resources accordingly.

A Profile is the mechanism for automated, fleet-wide rollouts: rather than manually binding each Target to a Release, an operator defines a Profile that continuously keeps the binding set in sync with the set of matching Targets.

## Architecture

```mermaid
flowchart TD
    subgraph Kubernetes
        Ctrl[Profile Controller]
        Prof[Profile]
        T1[Target A]
        T2[Target B]
        T3[Target C ✗]
        RB1[ReleaseBinding A]
        RB2[ReleaseBinding B]
        Rel[Release]
    end

    Ctrl -->|reconciles| Prof
    Prof -->|evaluates targetSelector| T1
    Prof -->|evaluates targetSelector| T2
    Prof -->|no match| T3

    Prof -->|creates / owns| RB1
    Prof -->|creates / owns| RB2

    RB1 -->|binds Target A to| Rel
    RB2 -->|binds Target B to| Rel
```

## Resource Owner References

```mermaid
flowchart LR
    subgraph Profile
        P[Profile]
    end

    subgraph Owned Resources
        RB1[ReleaseBinding A]
        RB2[ReleaseBinding B]
    end

    P -->|owns| RB1
    P -->|owns| RB2
```

ReleaseBindings are created with an owner reference to the Profile. Kubernetes garbage-collects them automatically when the Profile is deleted.

## Status Fields

| Field              | Description                                      |
| ------------------ | ------------------------------------------------ |
| `matchedTargets`   | Number of Targets currently matched by the selector |

## Watch Triggers

The Profile controller is triggered when:

- A `Profile` resource is created, updated, or deleted.
- A `ReleaseBinding` owned by the Profile changes (via `Owns`).
- A `Target` in the same namespace changes — the controller re-evaluates all Profiles whose selector might match the changed Target.

## ReleaseBinding Naming

ReleaseBindings are created with `generateName` using the pattern:

```
<profile-name>-<target-name>-<random-suffix>
```

Names are truncated to 57 characters before the suffix to stay within the 63-character Kubernetes label value limit.

## Relationship to Other Controllers

```mermaid
flowchart LR
    ProfileCtrl[Profile Controller] -->|creates / deletes| ReleaseBinding
    ReleaseBinding -->|triggers| TargetCtrl[Target Controller]
    TargetCtrl -->|creates| RenderTask
```

Deleting a Profile cascades into:
1. Kubernetes GC removes owned ReleaseBindings.
2. Target controller notices the missing bindings and stops managing the corresponding RenderTasks.

