# Quick Start

To try out SolAr, you can install it and go through the [walk-through](./walk-through/index.md).

## Quick Start Installation Methods

!!! info "Disclaimer for production use"

    These instructions are intended to help you get started quickly. They are not suitable for production. For production installs, please refer to the [installation documentation](./operator-manual/installation/installation.md).

### Dev Cluster

`make dev-cluster` needs the following tools installed and on your `PATH`:

- [Docker](https://docs.docker.com/get-docker/) (daemon running) — builds images and backs the Kind cluster
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) — creates the local Kubernetes cluster
- [kubectl](https://kubernetes.io/docs/tasks/tools/) — interacts with the cluster
- [Helm](https://helm.sh/docs/intro/install/) — installs cert-manager's trust bundle, Zot, and SolAr
- [Flux CLI](https://fluxcd.io/flux/installation/#install-the-flux-cli) — installs and verifies Flux
- [yq](https://github.com/mikefarah/yq#install) — extracts certs from cluster secrets

Everything else the target needs (e.g. the [OCM CLI](https://ocm.software/)) is installed automatically into `bin/` on first run.

Checkout the [SolAr Project](https://github.com/opendefensecloud/solution-arsenal) and run the make target `make dev-cluster`:

```shell
git clone https://github.com/opendefensecloud/solution-arsenal.git solar
cd solar
make dev-cluster
```

Afterwards you can interact with solar in the created kind cluster using `kubectl`.

Read more about the [local cluster with kind](./developer-guide/dev-cluster-with-kind.md).

### Helm

To quickly install SolAr on your own Kubernetes cluster you can use Helm:

You will need to ensure [cert-manager](https://cert-manager.io/docs/installation) and [Flux](https://fluxcd.io/flux/installation/) (`source-controller` and `helm-controller`) are installed in the cluster — Flux is what reconciles the `OCIRepository`/`HelmRelease` resources SolAr renders, so releases won't roll out without it. If your registries use a private CA, also install [trust-manager](https://cert-manager.io/docs/trust/trust-manager/) and set `caBundle.enabled=true`.

```shell
helm install solar oci://ghcr.io/opendefensecloud/charts/solar \
    --namespace solar-system \
    --create-namespace
```

See the [Helm installation reference](./operator-manual/installation/helm.md) for version pinning, custom values, and the full chart README.
