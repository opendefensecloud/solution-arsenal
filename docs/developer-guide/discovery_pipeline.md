# Discovery Pipeline Documentation

## Overview

The SolAr discovery pipeline (`solar-discovery`) is a standalone component that discovers OCM (Open Component Model) packages in OCI registries and writes them into the SolAr API as `Component` and `ComponentVersion` resources.

Discovery is triggered in two ways: by periodically scanning a registry for all repositories, or by receiving push notifications from the registry via webhook. These modes can be used together or independently (`scanInterval: 0` disables polling).

The pipeline is composed of a chain of channel-connected stages. Each stage processes events from its input channel and publishes results to its output channel, making the pipeline fully asynchronous and back-pressure-aware.

## Pipeline Stages

```mermaid
flowchart LR
    Reg[(OCI Registry)]
    Webhook[Webhook Server]
    Scanner[Registry Scanner]
    Chan([repoEvents channel])

    subgraph Pipeline
        Q[Qualifier]
        F[Filter]
        H[Handler]
        W[APIWriter]
    end

    K8s[(SolAr API)]

    Reg -->|poll interval| Scanner
    Reg -->|push notification| Webhook
    Scanner -->|RepositoryEvent| Chan
    Webhook -->|RepositoryEvent| Chan
    Chan --> Q
    Q -->|ComponentVersionEvent| F
    F -->|ComponentVersionEvent| H
    H -->|WriteAPIResourceEvent| W
    W -->|create/update/delete| K8s
```

### Event Sources

| Source          | Output            | Responsibility                                                          |
| --------------- | ----------------- | ----------------------------------------------------------------------- |
| RegistryScanner | `RepositoryEvent` | Periodically scans all repositories in an OCI registry                  |
| WebhookServer   | `RepositoryEvent` | Accepts push notifications from OCI registries and emits events directly |

### Pipeline Stages

| Stage     | Input                   | Output                  | Responsibility                                                                   |
| --------- | ----------------------- | ----------------------- | -------------------------------------------------------------------------------- |
| Qualifier | `RepositoryEvent`       | `ComponentVersionEvent` | Resolves repository name to namespace + component, looks up all versions via OCM |
| Filter    | `ComponentVersionEvent` | `ComponentVersionEvent` | Drops events for ComponentVersions that already exist in the cluster             |
| Handler   | `ComponentVersionEvent` | `WriteAPIResourceEvent` | Fetches the OCM component descriptor and builds the API resource payload         |
| APIWriter | `WriteAPIResourceEvent` | –                       | Creates, updates, or deletes `Component` and `ComponentVersion` resources        |

## Event Types

```mermaid
classDiagram
    class RepositoryEvent {
        Registry
        Repository
        Version
        Digest
        Type
    }

    class ComponentVersionEvent {
        Namespace
        Component
        Version
    }

    class WriteAPIResourceEvent {
        ComponentSpec
        HelmDiscovery
        Type
    }

    RepositoryEvent --> ComponentVersionEvent : Qualifier
    ComponentVersionEvent --> WriteAPIResourceEvent : Handler
```

## Registry Scanner

The `RegistryScanner` scans a single OCI registry on a configurable interval (`ScanInterval`, default 24 h). It lists all repositories via the ORAS library and emits a `RepositoryEvent` for each one. Concurrent scans are prevented by a mutex — if a scan is still running when the next tick fires, the tick is skipped.

OCI registries can also push change notifications via webhooks. The `WebhookServer` accepts HTTP POST requests on configured paths and converts them directly into `RepositoryEvent`s, bypassing the polling interval.

## Qualifier

The Qualifier resolves a raw `RepositoryEvent` (registry + repository path) into one or more `ComponentVersionEvent`s by:

1. Splitting the repository path into `namespace/component` segments.
2. If the event already carries a specific version (e.g. from a webhook), emitting a single event for that version.
3. Otherwise, looking up all versions of the component in the OCM repository and emitting one event per version.

## Filter

The Filter prevents duplicate work. For `EventCreated` events it checks whether the corresponding `ComponentVersion` already exists in the SolAr API. If it does, the event is silently dropped. All other event types (update, delete) pass through unconditionally.

## Handler

The Handler fetches the OCM component descriptor for a component version and builds the `ComponentVersion` payload. Currently handles components that contain exactly one Helm chart resource. Components with zero or more than one Helm chart are not yet supported.

## APIWriter

The APIWriter creates, updates, or deletes `Component` and `ComponentVersion` resources in the SolAr API. On deletion, if no more versions of a component remain, the parent `Component` resource is also deleted.

## Sequence Diagrams

### Scanner: Periodic poll discovers changes

```mermaid
sequenceDiagram
    participant Timer as Tick (interval)
    participant Scanner as Registry Scanner
    participant Reg as OCI Registry
    participant Chan as repoEvents channel

    Timer->>Scanner: tick
    Scanner->>Reg: List repositories
    Reg-->>Scanner: [ocm-demo, other-component, …]
    loop for each repository
        Scanner->>Chan: RepositoryEvent(repo=ocm-demo, version="")
    end
```

### Webhook: Registry pushes a change notification

```mermaid
sequenceDiagram
    participant Dev as Solution Maintainer
    participant Reg as OCI Registry
    participant Webhook as Webhook Server
    participant Chan as repoEvents channel

    Dev->>Reg: ocm transfer ctf ./ocm-demo-ctf localhost/test
    Note over Reg: Pushes opendefense.cloud/ocm-demo v26.4.1
    Reg->>Webhook: POST /webhook/zot-discovery<br/>(repo=…/ocm-demo, tag=v26.4.1)
    Webhook->>Chan: RepositoryEvent(version=v26.4.1)
```

### Shared pipeline: qualify, filter & write

Once a `RepositoryEvent` is on the channel, both the scanner and webhook paths converge into this shared pipeline starting with the Qualifier.

```mermaid
sequenceDiagram
    participant Chan as repoEvents channel
    participant Qualifier as Qualifier
    participant Reg as OCI Registry
    participant Filter as Filter
    participant Handler as Handler
    participant Writer as APIWriter
    participant K8s as SolAr API

    Chan->>Qualifier: RepositoryEvent(ocm-demo, version="")
    Note over Qualifier: No version specified —<br/>look up all versions
    Qualifier->>Reg: ListComponentVersions(ocm-demo)
    Reg-->>Qualifier: [v26.4.1, v26.4.0, …]
    loop for each version
        Qualifier->>Filter: ComponentVersionEvent(ocm-demo, vX.Y.Z)
    end
    Filter->>K8s: Get ComponentVersion "…-v26-4-1"
    K8s-->>Filter: NotFound
    Filter->>Handler: pass event
    Handler->>Reg: LookupComponentVersion(ocm-demo, v26.4.1)
    Reg-->>Handler: ComponentDescriptor (1 Helm resource)
    Handler->>Writer: WriteAPIResourceEvent(ComponentSpec)
    Writer->>K8s: Ensure Component "opendefense-cloud-ocm-demo"
    Writer->>K8s: Create ComponentVersion "…-v26-4-1"
```

### Component version deleted from registry (with Component cascade)

```mermaid
sequenceDiagram
    participant Dev as Solution Maintainer
    participant Reg as OCI Registry
    participant Webhook as Webhook Server

    Dev->>Reg: Delete OCI tag v26.4.1
    Reg->>Webhook: POST /webhook/zot-discovery<br/>(type=Deleted, digest=sha256:abc123…)
    Note over Webhook: Only webhooks carry deletion events.<br/>The scanner emits EventCreated only.
```

#### Pipeline passthrough & cascade delete

```mermaid
sequenceDiagram
    participant Qualifier as Qualifier
    participant Filter as Filter
    participant Handler as Handler
    participant Writer as APIWriter
    participant K8s as SolAr API

    Note over Qualifier: RepositoryEvent(type=Deleted) —<br/>no OCM lookup needed
    Qualifier->>Filter: ComponentVersionEvent(type=Deleted)
    Note over Filter: Deletions bypass exists-check
    Filter->>Handler: pass through
    Note over Handler: No descriptor fetch needed
    Handler->>Writer: WriteAPIResourceEvent(type=Deleted, digest=sha256:abc123…)
    Writer->>K8s: List CVs (label: digest=abc123…)
    K8s-->>Writer: [ocm-demo-v26-4-1]
    Writer->>K8s: Delete ComponentVersion
    Writer->>K8s: List CVs (label: component=ocm-demo)
    K8s-->>Writer: [] (none remaining)
    Writer->>K8s: Delete Component
```

## Configuration

The discovery pipeline is configured via the `solar-discovery` binary flags and a registry configuration file. Per-registry settings include:

| Setting          | Description                                             |
| ---------------- | ------------------------------------------------------- |
| `scanInterval`   | Polling interval for registry scans (0 = webhook only)  |
| `webhookPath`    | HTTP path to register for push notifications (optional) |
| `credentials`    | Username/password for authenticated registries          |
| `plainHTTP`      | Whether to use plain HTTP instead of HTTPS              |

