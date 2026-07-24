# Quick Start

To try out SolAr, you can install it and go through the [walk-through](./walk-through/index.md).

## Quick Start Installation Methods

!!! info "Disclaimer for production use"

    These instructions are intended to help you get started quickly. They are not suitable for production. For production installs, please refer to the [installation documentation](./operator-manual/installation/installation.md).

### Dev Cluster

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

You will need to ensure [cert-manager](https://cert-manager.io/docs/installation) is installed in the cluster.

```shell
helm install solar oci://ghcr.io/opendefensecloud/charts/solar \
  --namespace solar-system \
  --create-namespace
```

See the [Helm installation reference](./operator-manual/installation/helm.md) for version pinning, custom values, and the full chart README.
