# ComponentVersion Controller Documentation

## Overview

The ComponentVersion controller manages the self-finalizer on each `ComponentVersion` so its deletion is observable by other controllers.

## Architecture

```mermaid
flowchart TD
    subgraph Kubernetes
        Ctrl[ComponentVersion Controller]
        CV[ComponentVersion]
    end

    Ctrl -->|reconciles| CV
```

## Finalizers

| Finalizer | On resource | Purpose |
|---|---|---|
| `solar.opendefense.cloud/componentversion-finalizer` | ComponentVersion | Allows controllers to observe deletion before the object is garbage-collected |

On deletion, the controller removes `solar.opendefense.cloud/componentversion-finalizer` from the ComponentVersion, allowing it to be garbage-collected.

## Watch Triggers

The ComponentVersion controller is triggered when:

- A `ComponentVersion` resource is created, updated, or deleted.

## Relationship to Other Controllers

```mermaid
flowchart LR
    Release -->|references| ComponentVersion
    ReleaseCtrl[Release Controller] -->|protects| ComponentVersion
```

ComponentVersions are themselves protected from deletion by the Release controller: a ComponentVersion cannot be deleted while a Release references it. Once the last Release is removed, the ComponentVersion can be deleted.
