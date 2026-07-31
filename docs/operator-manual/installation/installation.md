# Overview

## Non-production installation

If you just want to try out SolAr in a non-production environment (including on desktop via minikube/kind/k3d etc) follow the [quick-start guide](../../getting-started.md).

## Production installation

### Prerequisites

SolAr requires [cert-manager](https://cert-manager.io/docs/installation/) as a dependency.

SolAr also requires [Flux](https://fluxcd.io/flux/installation/)'s `source-controller` and `helm-controller` to be installed: SolAr's renderer produces `OCIRepository`/`HelmRelease` resources, and Flux is what actually reconciles them into a deployed release. Without Flux running, releases render but never roll out.

If your OCI registries (component/chart sources) use a private or self-signed CA, also install [trust-manager](https://cert-manager.io/docs/trust/trust-manager/) and set `caBundle.enabled=true` on the SolAr chart so the controller trusts that CA.

### Installation Methods

To install SolAr, navigate to the [releases page](https://github.com/opendefensecloud/solution-arsenal/releases) and find the release you wish to use (the latest full release is preferred).

#### Helm

See [Helm installation](./helm.md) for more information.
