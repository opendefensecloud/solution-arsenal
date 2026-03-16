# Quick Start

To try out SolAr, you can install it and go through the [walk-through](./walk-through/about.md).

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

### Kustomize

To quickly install SolAr on your own kubernetes cluster you can use kustomize:

You will need to ensure [cert-manager](https://cert-manager.io/docs/installation) is installed in the cluster.

First, specify the version you want to install in an environment variable.

```shell
SOLAR_VERSION="main"
```

Then, copy the commands below to apply the kustomization:

```shell
kubectl apply -k "https://github.com/opendefensecloud/solution-arsenal/examples/deployment?ref=${SOLAR_VERSION}"
```
