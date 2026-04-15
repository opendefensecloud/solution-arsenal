# Discovery Pipeline Documentation

## Overview

The SolAr discovery pipeline (`solar-discovery`) is a standalone component that continuously scans OCI registries for OCM (Open Component Model) packages and writes the discovered components into the SolAr API as `Component` and `ComponentVersion` resources.

The pipeline is composed of a chain of typed, channel-connected stages. Each stage runs as a goroutine pool (`Runner`) that reads from an input channel, processes events, and publishes results to an output channel. This makes the pipeline fully asynchronous, back-pressure-aware, and easy to test by swapping individual processors.

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

### Stage Descriptions

| Stage           | Input                    | Output                   | Responsibility                                                                 |
| --------------- | ------------------------ | ------------------------ | ------------------------------------------------------------------------------ |
| RegistryScanner | –                        | `RepositoryEvent`        | Periodically scans or receives webhook calls for repository changes            |
| Qualifier       | `RepositoryEvent`        | `ComponentVersionEvent`  | Resolves repository name to namespace + component, looks up all versions via OCM |
| Filter          | `ComponentVersionEvent`  | `ComponentVersionEvent`  | Drops events for ComponentVersions that already exist in the cluster           |
| Handler         | `ComponentVersionEvent`  | `WriteAPIResourceEvent`  | Fetches the OCM component descriptor and builds the API resource payload       |
| APIWriter       | `WriteAPIResourceEvent`  | –                        | Creates, updates, or deletes `Component` and `ComponentVersion` resources      |

## Event Types

```mermaid
classDiagram
    class RepositoryEvent {
        +Timestamp time.Time
        +Registry string
        +Repository string
        +Type EventType
        +Version string
        +Digest string
    }

    class ComponentVersionEvent {
        +Timestamp time.Time
        +Source RepositoryEvent
        +Namespace string
        +Component string
    }

    class WriteAPIResourceEvent {
        +Timestamp time.Time
        +Source ComponentVersionEvent
        +HelmDiscovery HelmDiscovery
        +ComponentSpec compdesc.ComponentSpec
    }

    RepositoryEvent --> ComponentVersionEvent : Qualifier
    ComponentVersionEvent --> WriteAPIResourceEvent : Handler
```

## Registry Scanner

The `RegistryScanner` scans a single OCI registry on a configurable interval (default 30 s). It lists all repositories via the ORAS library and emits a `RepositoryEvent` for each one. Concurrent scans are prevented by a mutex — if a scan is still running when the next tick fires, the tick is skipped.

OCI registries can also push change notifications via webhooks. The `WebhookServer` accepts HTTP POST requests on configured paths and converts them directly into `RepositoryEvent`s, bypassing the polling interval.

## Qualifier

The Qualifier resolves a raw `RepositoryEvent` (registry + repository path) into one or more `ComponentVersionEvent`s by:

1. Splitting the repository path into `namespace/component` segments.
2. If the event already carries a specific version (e.g. from a webhook), emitting a single event for that version.
3. Otherwise, looking up all versions of the component in the OCM repository and emitting one event per version.

## Filter

The Filter prevents duplicate work. For `EventCreated` events it checks whether the corresponding `ComponentVersion` already exists in the SolAr API. If it does, the event is silently dropped. All other event types (update, delete) pass through unconditionally.

## Handler

The Handler fetches the full OCM component descriptor for the component version and determines how to represent it as a SolAr API resource. It uses a pluggable handler registry keyed by `HandlerType`:

| HandlerType | Trigger condition                                  | Produces                                  |
| ----------- | -------------------------------------------------- | ----------------------------------------- |
| `helm`      | Component descriptor contains exactly 1 Helm chart | `ComponentVersion` with Helm resource     |

Components with zero or more than one Helm chart are not yet handled and produce an error.

## APIWriter

The APIWriter translates `WriteAPIResourceEvent`s into Kubernetes API calls against the SolAr extension API:

- **Create / Update**: calls `ensureComponent` (idempotent upsert of the `Component` parent resource) and then creates or patches the `ComponentVersion`.
- **Delete**: deletes the `ComponentVersion` and, if no more versions exist, the `Component`.

Labels `solar.opendefense.cloud/component` and `solar.opendefense.cloud/digest` are set on `ComponentVersion` resources for indexing and deduplication.

## Runner: Shared Concurrency Primitive

All pipeline stages are backed by the generic `Runner[InputEvent, OutputEvent]` type from `pkg/discovery`. Each Runner runs as a single goroutine that:

- Reads events from its input channel in a single goroutine loop.
- Calls `Processor.Process(ctx, event)` to produce zero or more output events.
- Publishes output events to its output channel.
- Supports optional **rate limiting** (`rate.Limiter`) to throttle API calls.
- Supports optional **exponential backoff** for transient errors (e.g. registry 429s).
- Can be stopped gracefully: `Stop()` closes the stop channel and waits for the in-flight event to finish.

## Sequence Diagrams

### Webhook: Registry pushes a change notification

```mermaid
sequenceDiagram
    participant Dev as Solution Maintainer
    participant Reg as zot-discovery (OCI)
    participant Webhook as Webhook Server
    participant Qualifier as Qualifier
    participant Filter as Filter
    participant Handler as Handler
    participant Writer as APIWriter
    participant K8s as SolAr API

    Dev->>Reg: ocm transfer ctf ./ocm-demo-ctf localhost/test
    Note over Reg: Pushes opendefense.cloud/ocm-demo v26.4.1
    Reg->>Webhook: POST /webhooks/zot-discovery<br/>(repo=test/component-descriptors/opendefense.cloud/ocm-demo, tag=v26.4.1)
    Webhook->>Qualifier: RepositoryEvent(version=v26.4.1)

    Note over Qualifier: Version specified — emit single event directly
    Qualifier->>Filter: ComponentVersionEvent(opendefense.cloud/ocm-demo, v26.4.1)

    Filter->>K8s: Get ComponentVersion "opendefense-cloud-ocm-demo-v26-4-1"
    K8s-->>Filter: NotFound
    Filter->>Handler: ComponentVersionEvent(opendefense.cloud/ocm-demo, v26.4.1)

    Handler->>Reg: LookupComponentVersion(opendefense.cloud/ocm-demo, v26.4.1)
    Reg-->>Handler: ComponentDescriptor (1 Helm resource)
    Handler->>Writer: WriteAPIResourceEvent(ComponentSpec)

    Writer->>K8s: Ensure Component "opendefense-cloud-ocm-demo"
    Writer->>K8s: Create ComponentVersion "opendefense-cloud-ocm-demo-v26-4-1"
```

### Component version deleted from registry (with Component cascade)

```mermaid
sequenceDiagram
    participant Dev as Solution Maintainer
    participant Reg as zot-discovery (OCI)
    participant Webhook as Webhook Server
    participant Qualifier as Qualifier
    participant Filter as Filter
    participant Handler as Handler
    participant Writer as APIWriter
    participant K8s as SolAr API

    Dev->>Reg: Delete OCI tag v26.4.1<br/>(repo: test/component-descriptors/opendefense.cloud/ocm-demo)
    Reg->>Webhook: POST /webhook/zot-discovery<br/>(type=Deleted, digest=sha256:abc123...)
    Note over Webhook: Only webhooks carry deletion events and digest.<br/>The scanner always emits EventCreated only.
    Webhook->>Qualifier: RepositoryEvent(type=Deleted, digest=sha256:abc123...)
    Note over Qualifier: Deletion — pass through without OCM lookup
    Qualifier->>Filter: ComponentVersionEvent(type=Deleted)
    Note over Filter: Deletions bypass the exists-check
    Filter->>Handler: ComponentVersionEvent(type=Deleted)
    Note over Handler: Return immediately — no descriptor fetch needed
    Handler->>Writer: WriteAPIResourceEvent(type=Deleted, digest=sha256:abc123...)

    Writer->>K8s: List ComponentVersions<br/>(label: digest=abc123...)
    K8s-->>Writer: [opendefense-cloud-ocm-demo-v26-4-1]
    Writer->>K8s: Delete ComponentVersion "opendefense-cloud-ocm-demo-v26-4-1"
    Note over Writer: Check remaining ComponentVersions for parent Component
    Writer->>K8s: List ComponentVersions<br/>(label: component=opendefense-cloud-ocm-demo)
    K8s-->>Writer: [] (none remaining)
    Writer->>K8s: Delete Component "opendefense-cloud-ocm-demo"
```

## Configuration

The discovery pipeline is configured via the `solar-discovery` binary flags and a registry configuration file. Per-registry settings include:

| Setting          | Description                                             |
| ---------------- | ------------------------------------------------------- |
| `scanInterval`   | Polling interval for registry scans (0 = webhook only)  |
| `webhookPath`    | HTTP path to register for push notifications (optional) |
| `credentials`    | Username/password for authenticated registries          |
| `plainHTTP`      | Whether to use plain HTTP instead of HTTPS              |

