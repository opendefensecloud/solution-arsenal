# Discovery Pipeline Documentation

## Overview

The SolAr discovery pipeline (`solar-discovery`) is a standalone component that discovers OCM (Open Component Model) packages in OCI registries and writes them into the SolAr API as `ComponentVersion` resources.

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
    W -->|create/update/annotate| K8s
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
| APIWriter | `WriteAPIResourceEvent` | –                       | Creates or updates `ComponentVersion` resources; on delete events annotates them for an availability re-check |

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
        Source
        Namespace
        Component
    }

    class WriteAPIResourceEvent {
        Source
        Component
        HelmDiscovery
    }

    RepositoryEvent --> ComponentVersionEvent : Qualifier
    ComponentVersionEvent --> WriteAPIResourceEvent : Handler
```

`Source` retains the originating event: version, digest, and event type are
available through `ComponentVersionEvent.Source` and
`WriteAPIResourceEvent.Source.Source`. `WriteAPIResourceEvent.Component`
is a `descruntime.Component`.

## Registry Scanner

The `RegistryScanner` scans a single OCI registry immediately on startup and then at the positive interval configured in `Registry.spec.scanInterval`. With an unset or zero interval, the pipeline does not start a scanner. It lists repositories via ORAS and emits a `RepositoryEvent` only for paths matching the OCM component-descriptor layout. Concurrent scans are prevented by a mutex — if a scan is still running when the next tick fires, the tick is skipped.

OCI registries can also push change notifications via webhooks. The `WebhookServer` accepts HTTP POST requests on configured paths and converts them directly into `RepositoryEvent`s, bypassing the polling interval.

## Qualifier

The Qualifier resolves a raw `RepositoryEvent` (registry + repository path) into one or more `ComponentVersionEvent`s by:

1. Splitting `<base>/component-descriptors/<component>` into the repository namespace (`<base>`) and component name. The base may be empty; unrelated repository paths are rejected.
2. If the event already carries a specific version (e.g. from a webhook), emitting a single event for that version.
3. Otherwise, looking up all versions of the component in the OCM repository and emitting one event per version.

Deletion events pass through without an OCM lookup.

## Filter

The Filter prevents duplicate work. For `EventCreated` events it checks whether the corresponding `ComponentVersion` already exists in the SolAr API. If it does, the event is silently dropped. All other event types (update, delete) pass through unconditionally.

## Handler

The Handler fetches the OCM component descriptor for a component version and builds the `ComponentVersion` payload. Only components that satisfy the [OCM packaging contract](../user-guide/ocm-packaging.md) are processed.

The descriptor is fetched through `ocmv2.Repository.GetComponentVersion`. The handler passes its `descruntime.Component` and discovered Helm metadata to the APIWriter.

## APIWriter

The APIWriter creates or updates `ComponentVersion` resources in the SolAr API. Each ComponentVersion carries the OCM component name and its source `scheme`, `registry` and `repository`. Each ComponentVersion also records the descriptor's full manifest digest in `spec.digest` and starts with `Available=True`; from then on the ComponentVersion controller owns availability.

An OCM component version is immutable, so an update whose descriptor digest differs from the recorded one is refused and only logged: the tag was re-pushed with other content. The ComponentVersion keeps pointing at the descriptor it was discovered from, which the controller then reports as missing. Delete the ComponentVersion to re-discover the new content. The scan path reaches the same outcome because the Filter never updates an existing ComponentVersion.

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
    Reg-->>Scanner: [test/component-descriptors/opendefense.cloud/ocm-demo, other-repo, …]
    loop for each OCM descriptor repository
        Scanner->>Chan: RepositoryEvent(repo=test/component-descriptors/opendefense.cloud/ocm-demo, version="")
    end
```

### Webhook: Registry pushes a change notification

```mermaid
sequenceDiagram
    participant Dev as Solution Maintainer
    participant Reg as OCI Registry
    participant Webhook as Webhook Server
    participant Chan as repoEvents channel

    Dev->>Reg: ocm --config uploaders.yaml transfer cv<br/>ctf::./ocm-demo-ctf//opendefense.cloud/ocm-demo:v26.4.1 https://registry.example.com/test
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
    Handler->>Reg: GetComponentVersion(ocm-demo, v26.4.1)
    Reg-->>Handler: ComponentDescriptor (1 Helm resource)
    Handler->>Writer: WriteAPIResourceEvent(Component)
    Writer->>K8s: Create ComponentVersion "…-v26-4-1"
```

### Component version deleted from registry

```mermaid
sequenceDiagram
    participant Dev as Solution Maintainer
    participant Reg as OCI Registry
    participant Webhook as Webhook Server

    Dev->>Reg: Delete OCI tag v26.4.1
    Reg->>Webhook: POST /webhook/zot-discovery<br/>(type=Deleted, digest=sha256:abc123…)
    Note over Webhook: Only webhooks carry deletion events.<br/>Without one, the controller's periodic availability check notices the miss.
```

#### Pipeline passthrough & availability re-check

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
    Writer->>K8s: List CVs (fieldSelector: spec.digest=sha256:abc123…)
    K8s-->>Writer: [ocm-demo-v26-4-1]
    Writer->>K8s: Patch annotation availability-recheck=<timestamp>
    Note over K8s: ComponentVersion controller re-checks the manifest;<br/>Available goes Unknown, then False after the grace period
```

## Configuration

The `solar-discovery` binary uses `--namespace` to select the namespace in
which it watches `Registry` resources and credential Secrets, and `--listen`
to set the webhook listen address. Per-registry settings live in
`Registry.spec`:

| Setting | Description |
| ------- | ----------- |
| `hostname` | Registry host, optionally including a port |
| `scanInterval` | Positive polling interval, such as `30s`; unset or `0s` disables scanning |
| `webhookPath` | HTTP path for push notifications; unset disables webhooks |
| `flavor` | Registry type used for webhook routing, required with `webhookPath` |
| `solarSecretRef` | Reference to a credential Secret in the same namespace; omit for anonymous discovery |
| `plainHTTP` | Use HTTP instead of HTTPS; defaults to false |

The transfer example uses the OCM v2 uploader configuration described in the
[packaging guide](../user-guide/ocm-packaging.md#1-published-to-an-oci-registry).
