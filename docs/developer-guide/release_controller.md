# Release Controller Documentation

## Overview

The Release controller manages the lifecycle of `Release` custom resources in SolAr. Its primary responsibility is to validate that a Release's referenced `ComponentVersion` exists and to reflect the resolution status via status conditions.

Once the ComponentVersion is resolved, the controller also writes `status.effectiveUniqueName` — the deduplication key the Target controller will use for this Release. This equals `spec.uniqueName` when set, or the parent Component name derived from the ComponentVersion otherwise.

The Release controller does **not** trigger rendering — that is handled by the Target controller once a Release is bound to a Target via a `ReleaseBinding`.

## Architecture

```mermaid
flowchart TD
    subgraph Kubernetes
        Ctrl[Release Controller]
        Rel[Release]
        CV[ComponentVersion]
    end

    Ctrl -->|reconciles| Rel
    Rel -->|resolves| CV
    CV -->|ComponentRef.Name| Ctrl
    Ctrl -->|writes status.effectiveUniqueName| Rel
    Ctrl -->|adds/removes componentversion-ref| CV
```

## Finalizers

The Release controller manages two finalizers:

| Finalizer | On resource | Purpose |
|---|---|---|
| `solar.opendefense.cloud/release-finalizer` | Release | Allows the controller to observe deletion and run cleanup logic before the object is garbage-collected |
| `solar.opendefense.cloud/componentversion-ref` | ComponentVersion | Prevents deletion of the referenced ComponentVersion while any Release references it |

On deletion, the controller:

1. Checks whether any other active Release still references the same ComponentVersion.
2. If none remain, removes `solar.opendefense.cloud/componentversion-ref` from the ComponentVersion.
3. Removes `solar.opendefense.cloud/release-finalizer` from the Release, allowing it to be garbage-collected.

Cross-namespace references (resolved via `ReferenceGrant`) are considered when counting active references.

## Status Conditions

```mermaid
stateDiagram-v2
    [*] --> Unresolved: Release created
    Unresolved --> ComponentVersionResolved: ComponentVersion found
    Unresolved --> Unresolved: ComponentVersion missing (requeues on CV change)
    ComponentVersionResolved --> Unresolved: ReferenceGrant revoked
    ComponentVersionResolved --> [*]
```

| Condition                    | Status  | Reason      | Description                          |
| ---------------------------- | ------- | ----------- | ------------------------------------ |
| `ComponentVersionResolved`   | `True`  | `Resolved`  | ComponentVersion exists              |
| `ComponentVersionResolved`   | `False` | `NotFound`  | ComponentVersion does not exist      |
| `ComponentVersionResolved`   | `False` | `NotGranted`| Cross-namespace access not permitted by ReferenceGrant |

## Status Fields

| Field                    | Description                                                                                 |
| ------------------------ | ------------------------------------------------------------------------------------------- |
| `effectiveUniqueName`    | The deduplication key used by the Target controller. Equals `spec.uniqueName` when set, otherwise the parent Component name from the referenced ComponentVersion. `spec.uniqueName` itself is not modified — this field exists purely for operator visibility. |

## Watch Triggers

The Release controller is triggered when:

- A `Release` resource is created, updated, or deleted.
- A `ComponentVersion` that is referenced by one or more Releases changes.
- A `ReferenceGrant` that covers a cross-namespace ComponentVersion reference changes.

## Relationship to Other Controllers

The Release controller is intentionally minimal. Rendering logic is delegated to the Target controller:

```mermaid
flowchart LR
    Release -->|validated by| ReleaseCtrl[Release Controller]
    Release -->|referenced by| ReleaseBinding
    ReleaseBinding -->|bound by| Target
    Target -->|creates RenderTasks via| TargetCtrl[Target Controller]
```
