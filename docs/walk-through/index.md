# Walk-Through

This walk-through deploys an OCM-packaged application through SolAr's full pipeline: from registry discovery to a running workload on a target cluster.

## OCM, Flux CD, and Gitless GitOps

SolAr combines **Open Component Model (OCM)** for packaging with **Flux CD** for deployment, following a **gitless GitOps** pattern — no Git repository for deployment manifests.

**OCM** is the package format. A component version lives in a standard OCI registry and bundles a component descriptor (metadata, version, resource list), Helm charts, and container image references. SolAr's discovery pipeline scans the registry layout (`{namespace}/component-descriptors/{component-name}`) to find packages automatically.

**Flux CD** is the deployment engine. SolAr renders Helm charts that wrap Flux resources (`OCIRepository` + `HelmRelease`) and pushes them to an output registry. Flux on each target cluster pulls its desired state from that `OCIRepository` — not from a Git repo — and reconciles it continuously.

This means there is no manifest repository to maintain: SolAr produces deployment artifacts from Release and Target configuration and publishes them directly as OCI artifacts. Each target gets its own rendered chart, isolated by OCI reference rather than Git branch.

## Architecture

![Walk-through architecture](img/walkthrough-architecture.svg)

The walk-through follows three steps through SolAr's pipeline:

1. **Discovery** — SolAr scans an OCI registry for OCM component versions and creates `Component` / `ComponentVersion` resources in the Kubernetes API.
2. **Releases** — A `Release` references a `ComponentVersion`. SolAr renders a Helm chart containing the Flux resources needed to deploy it, and pushes the chart to the output registry.
3. **Bootstrap** — A `Target` binds releases to a cluster. SolAr creates a `Bootstrap` with a rendered chart that bundles all the target's releases. Flux on the target cluster picks up the chart and deploys the application.

## Prerequisites

You need a running dev cluster with SolAr and its dependencies installed. See [Getting Started](../getting-started.md) for setup instructions.

## Steps

1. [Discovery](01-discovery.md) — Set up a Discovery resource, transfer an OCM component to the registry, and verify SolAr discovers it.
2. [Releases](02-releases.md) — Create a Release from the discovered ComponentVersion and inspect the rendered chart.
3. [Bootstrap](03-bootstrap.md) — Register a Target, apply the Flux HelmRelease, and confirm the application is running.

For a complete description of SolAr's architecture, see the [Architecture documentation](../developer-guide/architecture.md) and [ADRs](../developer-guide/adrs/).
