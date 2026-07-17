# SolAr Discovery

SolAr Discovery is a standalone tool that scans OCI registries for
[Open Component Model (OCM)](https://ocm.software) packages and populates the
SolAr catalog by creating `Component` and `ComponentVersion` resources in a
Kubernetes cluster.

Discovery is **fully optional** — the SolAr catalog can be populated through
other means (direct API calls, GitOps, catalog chaining). See
[ADR-008](../developer-guide/adrs/008-No-Auth-Architecture.md), principle 6.

## Operating Modes

Discovery supports two operating modes that can be used independently or
combined on the same registry.

### Scan Mode

In scan mode, discovery periodically performs a full scan of the registry,
walking all repositories to find OCM component descriptors. This is the
simplest mode and works with any OCI registry.

```yaml
registries:
  - name: my-registry
    hostname: registry.example.com
    scanInterval: 24h
```

### Webhook Mode

In webhook mode, discovery listens for HTTP notifications from the registry.
When a new image is pushed or deleted, the registry sends an event and
discovery processes it immediately. This provides near-real-time catalog
updates.

Webhook mode requires a registry that supports event notifications (e.g. Zot).

```yaml
registries:
  - name: my-registry
    hostname: registry.example.com
    webhookPath: events
    flavor: zot
```

### Combined Mode

Both modes can be enabled on the same registry. The scan provides a baseline
and catches anything the webhook might miss; the webhook provides real-time
updates between scans.

```yaml
registries:
  - name: my-registry
    hostname: registry.example.com
    scanInterval: 24h
    webhookPath: events
    flavor: zot
```

## Installation

### Helm Chart

The recommended way to deploy discovery in a Kubernetes cluster:

```bash
helm upgrade --install solar-discovery oci://ghcr.io/opendefensecloud/charts/solar-discovery \
  --namespace solar-system \
  --set namespace=solar-system \
  --values my-values.yaml
```

### Binary

Discovery can also be run as a standalone binary outside a cluster:

```bash
solar-discovery --config config.yaml --namespace default
```

When running outside a cluster, set the `KUBECONFIG` environment variable to
point to a kubeconfig file with access to the target cluster's SolAr API.

## Configuration

### Config File Format

The config file is a YAML file listing the registries to scan:

```yaml
registries:
  - name: production
    hostname: registry.example.com
    scanInterval: 24h
    credentials:
      username: ${REGISTRY_USERNAME}
      password: ${REGISTRY_PASSWORD}

  - name: staging
    hostname: staging-registry.example.com
    scanInterval: 1h
    plainHTTP: true
```

### Environment Variable Substitution

The config file supports `$VAR` and `${VAR}` syntax for environment variable
expansion. This is the recommended way to inject credentials without storing
them in plaintext:

```yaml
registries:
  - name: my-registry
    hostname: registry.example.com
    credentials:
      username: ${REGISTRY_USERNAME}
      password: ${REGISTRY_PASSWORD}
```

### Registry Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | yes | — | Unique local identifier for this registry |
| `hostname` | string | yes | — | Registry hostname and optional port |
| `scanInterval` | duration | no | — | How often to run a full scan; leave unset to disable scan mode |
| `webhookPath` | string | no | — | Webhook endpoint path (enables webhook mode) |
| `flavor` | string | no | — | Webhook implementation (e.g. `zot`) |
| `plainHTTP` | bool | no | `false` | Use HTTP instead of HTTPS |
| `credentials.username` | string | no | — | Registry username |
| `credentials.password` | string | no | — | Registry password |

### CLI Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--config` | `-c` | — | Path to the registry config file (required) |
| `--namespace` | `-n` | `default` | Kubernetes namespace for Component/ComponentVersion resources |
| `--listen` | `-l` | `0.0.0.0:8080` | Address for the webhook HTTP listener |

### Helm Chart Values

See `charts/solar-discovery/values.yaml` for the full list of configurable
values. Key settings:

| Value | Description |
|-------|-------------|
| `registries` | Optional list of Registry CRs to create alongside the release; see [Registry lifecycle](#registry-lifecycle-embedded-vs-standalone) below |
| `namespace` | Target namespace for discovered resources |
| `envFrom` | Secret/ConfigMap references for environment variables |
| `caBundle.enabled` | Mount a CA bundle ConfigMap for TLS connections |
| `caBundle.configMapName` | Name of the CA bundle ConfigMap |
| `service.enabled` | Create a Service for webhook mode |
| `rbac.create` | Create ClusterRole/ClusterRoleBinding for API access |

### Registry lifecycle: embedded vs standalone

When deploying discovery via the Helm chart, Registry CRs can be
declared in two places. Pick per your scenario.

**Embedded (via `.Values.registries`)** — recommended for demo,
single-tenant, and quick-start deployments:

```yaml
# values.yaml — chart creates and owns these Registries
registries:
  - name: zot-source
    hostname: zot.example.com:5000
    flavor: zot
    webhookPath: events
    solarSecretRef: zot-source-auth   # Secret must already exist in-namespace
    targetPullSecretName: regcred

namespace: solar-system
```

Chart-managed Registries carry the standard Helm labels
(`app.kubernetes.io/managed-by=Helm`,
`app.kubernetes.io/instance=<release>`) and **are deleted on
`helm uninstall`**. If you upgrade a Target or a
RegistryBinding to depend on a chart-managed Registry, be aware
that uninstalling the discovery release will break those
downstream references.

**Standalone (out-of-band `kubectl apply`)** — required for
multi-tenant setups where the same Registry is shared across
namespaces via `RegistryBinding` + `ReferenceGrant`. Leave
`.Values.registries` empty in the chart and apply Registry CRs
separately as part of the registry backend's own manifests. See
`test/fixtures/e2e/zot-discovery-registry-webhook.yaml` for a
canonical shape.

**Both cases** require the referenced `solarSecretRef` Secret to
exist in the namespace before the Registry is reconciled — the
chart references Secrets by name but does not create them.

## Examples

### Minimal Scan-Only Setup

```yaml
# values.yaml
registries:
  - name: main
    hostname: registry.example.com
    scanInterval: 1h

namespace: solar-system
service:
  enabled: false
```

### Webhook with Zot Registry

```yaml
# values.yaml
registries:
  - name: zot
    hostname: zot.internal:5000
    webhookPath: events
    flavor: zot
    credentials:
      username: ${username}
      password: ${password}

namespace: solar-system

envFrom:
  - secretRef:
      name: zot-credentials

caBundle:
  enabled: true
  configMapName: root-bundle
```

### Multiple Registries

```yaml
# values.yaml
registries:
  - name: production
    hostname: prod-registry.example.com
    scanInterval: 24h
    credentials:
      username: ${PROD_USERNAME}
      password: ${PROD_PASSWORD}

  - name: staging
    hostname: staging-registry.example.com
    scanInterval: 30m
    webhookPath: events
    flavor: zot
    credentials:
      username: ${STAGING_USERNAME}
      password: ${STAGING_PASSWORD}

namespace: solar-system

envFrom:
  - secretRef:
      name: registry-credentials
```

### Running Outside a Cluster

```bash
# Set kubeconfig for API access
export KUBECONFIG=~/.kube/config

# Set registry credentials
export REGISTRY_USERNAME=admin
export REGISTRY_PASSWORD=secret

# Run discovery
solar-discovery --config config.yaml --namespace solar-system
```

## See also

- [Helm values templating](helm-values-templating.md) — how OCM
  components can ship a Helm values template that discovery renders
  against the current registry.

