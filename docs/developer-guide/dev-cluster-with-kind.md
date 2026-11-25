# Development Cluster with Kind

This guide describes how to set up a local development cluster using [Kind](https://kind.sigs.k8s.io/) for testing and developing SolAr.

!!! warning

    This setup is intended for local development and testing only. Do not use in production.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed and running
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) installed
- [kubectl](https://kubernetes.io/docs/tasks/tools/) installed
- [Helm](https://helm.sh/docs/intro/install/) installed
- [yq](https://github.com/mikefarah/yq#install) installed
- [Flux CLI](https://fluxcd.io/flux/installation/#install-the-flux-cli) installed

This should be taken care of if you use the Makefile. The `ocm` CLI is not a
manual prerequisite, the Makefile provisions it into `bin/` automatically.

## Quick Start

Spin up the complete development cluster:

```bash
make dev-cluster
```

This will:

1. Create a Kind cluster named `solar-dev` (if it doesn't exist)
2. Build and load Docker images into the cluster
3. Transfer the ocm-demo component
4. Install and configure:
   - cert-manager
   - trust-manager
   - Zot registries (discovery and deploy)
   - SolAr with your local images
   - solar-discovery in scan mode, with the local discovery Zot already
     registered as a `Registry`, so the catalog populates automatically once an
     OCM package is pushed

## What Gets Installed

| Component     | Namespace    | Description                      |
| ------------- | ------------ | -------------------------------- |
| cert-manager  | cert-manager | TLS certificate management       |
| trust-manager | cert-manager | Trust bundle management          |
| zot-discovery | zot          | OCI registry for discovery       |
| zot-deploy    | zot          | OCI registry for deployment      |
| solar         | solar-system | SolAr API server and controllers |
| solar-discovery | solar-system | Scans the discovery registry (scan mode) and populates the catalog |

## Accessing Registries

The Zot registries use LoadBalancer services provided by
[cloud-provider-kind](https://kind.sigs.k8s.io/docs/user/loadbalancer/), started
by `hack/dev-cluster.sh`. Addresses are fixed in `test/fixtures/zot-services.yaml`:

| Registry | Address |
| --- | --- |
| zot-discovery | `172.30.255.200:443` |
| zot-deploy | `172.30.255.201:443` |
| zot-discovery-2 (chaining) | `172.30.255.202:443` |

Setup creates the `solar-kind` network (`172.30.0.0/16`), restricting automatic
addresses to `172.30.0.0/24` to reserve the registry IPs. It uses
`KIND_EXPERIMENTAL_DOCKER_NETWORK`, which triggers Kind's unsupported-feature
warning. Only one registry stack can use these IPs at a time across dev, UI
and E2E clusters.

Upload OCM content to the address reachable by its consumers. Host clients
must trust `test/fixtures/ca.crt` and reach the Docker network directly;
Docker Desktop and rootless setups may need extra routing.

For the commands below:

```bash
DISCOVERY_IP=172.30.255.200
DEPLOY_IP=172.30.255.201
```

### Pushing Images

Tag and push images to the local registries:

```bash
# Push to zot-discovery
docker tag localhost/local/solar-discovery-worker:dev.* \
    ${DISCOVERY_IP}:443/solar-discovery-worker:local
docker push ${DISCOVERY_IP}:443/solar-discovery-worker:local

# Push to zot-deploy
docker tag localhost/local/solar-apiserver:dev.* \
    ${DEPLOY_IP}:443/solar-apiserver:local
docker push ${DEPLOY_IP}:443/solar-apiserver:local
```

### Pulling Images

Configure your local Kubernetes cluster to pull from the local registries:

```bash
# Create image pull secret if needed
kubectl create secret docker-registry zot-creds \
    --docker-server=${DISCOVERY_IP}:443 \
    --docker-username=admin \
    --docker-password=admin \
    -n your-namespace

# Add to service account
kubectl patch serviceaccount default \
    -p '{"imagePullSecrets":[{"name":"zot-creds"}]}' \
    -n your-namespace
```

## Setting Up Discovery for Testing

The `test/fixtures/setup-discovery.sh` script sets up the OCM transfer workflow for testing the discovery worker.

### When to Use It

Use this script when you want to:

- Test the discovery worker with real OCI artifacts
- Verify discovery functionality end-to-end
- Debug discovery-related issues

### Running the Script

```bash
./test/fixtures/setup-discovery.sh
```

This will:

1. Wait for Zot and its fixed LoadBalancer address
2. Transfer the ocm-demo and released solar-agent components to
   `https://172.30.255.200:443/test`, using `test/fixtures/e2e/ocmconfig` for
   credentials and `SSL_CERT_FILE` for the cluster CA

Since `make dev-cluster` deploys solar-discovery in scan mode with the discovery
Zot already registered, you do not apply a `Registry` yourself. A few seconds
after the transfer the discovery worker scans the registry and creates the
`ComponentVersion` resources in the `solar-system` namespace:

```bash
kubectl --context kind-solar-dev -n solar-system get componentversions
```

### Environment Variables

| Variable           | Default                              | Description                                                                                                              |
| --                 | --                                   | --                                                                                                                       |
| `KIND_CLUSTER_DEV` | `solar-dev`                          | Kind cluster name                                                                                                        |
| `KUBECTL`          | `kubectl`                            | Kubernetes CLI                                                                                                           |
| `OCM`              | `ocm`                                | OCM v2 CLI. For standalone use from the repository root, pass `OCM=./bin/go/ocm2` (provisioned by Make, outside `PATH`). |
| `OCM_CONFIG`       | `./test/fixtures/e2e/ocmconfig`      | Registry credentials for the discovery LoadBalancer.                                            |
| `OCM_UPLOADERS`    | `./test/fixtures/ocm-uploaders.yaml` | Loaded alongside credentials to publish pullable images and charts. Keep `oci.uploader` before `localblob.uploader`.     |
| `SSL_CERT_FILE`    | `$PWD/test/fixtures/ca.crt`          | Cluster CA generated during setup. OCM v2 uses this instead of `rootcerts` config.                                       |
| `OCM_DEMO_DIR`     | `$PWD/test/fixtures/ocm-demo-ctf`    | Demo CTF location                                                                                                        |
| `OCM_AGENT_VERSION` | none, required                      | Version of the released solar-agent component. Pinned and exported by the Makefile.                                      |
| `OCM_AGENT_REPO`   | `ghcr.io/opendefensecloud`           | Where the solar-agent component is read from. Point it at a CTF to try an unreleased version, see below.                 |

Example:

```bash
OCM=./bin/go/ocm2 OCM_AGENT_VERSION=$(sed -n 's/^export OCM_AGENT_VERSION ?= //p' Makefile) \
  KIND_CLUSTER_DEV=my-cluster ./test/fixtures/setup-discovery.sh
```

## Setting Up Release for Testing

The `test/fixtures/setup-release.sh` script creates ComponentVersion and Release resources to test if RenderTasks are created correctly and can push to the zot-deploy registry.

### When to Use It

Use this script when you want to:

- Test Release resource creation and rendering
- Verify RenderTasks are created for Release resources
- Test pushing rendered releases to the deploy zot registry

### Prerequisites

Run `setup-discovery.sh` first to transfer the ocm-demo OCM package to zot-discovery:

```bash
./test/fixtures/setup-discovery.sh
```

### Running the Script

```bash
./test/fixtures/setup-release.sh
```

This will apply:

- `test/fixtures/e2e/componentversion.yaml` - Creates Component and ComponentVersion resources
- `test/fixtures/e2e/release.yaml` - Creates a Release resource

### Environment Variables

| Variable           | Default        | Description       |
| ------------------ | -------------- | ----------------- |
| `KIND_CLUSTER_DEV` | `solar-dev`    | Kind cluster name |
| `KUBECTL`          | `kubectl`      | Kubernetes CLI    |
| `NAMESPACE`        | `solar-system` | Target namespace  |

Example:

```bash
NAMESPACE=my-namespace ./test/fixtures/setup-release.sh
```

### Watching the Results

After applying, watch for the Release and its associated Job/Pod. Replace `my-namespace` with your namespace if different:

```bash
kubectl get componentversions,releases,jobs,pods -n my-namespace -w
```

The flow is:

1. **Component** and **ComponentVersion** are created in the namespace
2. **Release** is created in the namespace
3. The rendertask_controller creates a **Job** in the same namespace
4. The Job spawns a **Pod** that renders the release and pushes it to the zot-deploy registry


## Setting Up Solar Agent for Testing

The agent is delivered like any catalog component. `make demo-app` transfers the
released `opendefense.cloud/solar-agent` component (from
[opendefensecloud/ocm](https://github.com/opendefensecloud/ocm), version pinned
as `OCM_AGENT_VERSION` in the Makefile) into zot-discovery, and applies
`test/fixtures/e2e/agent-release.yaml` in `solar-system`: a `Release` annotated
`solar.opendefense.cloud/target-credential: "true"` and a `Profile` with an
empty selector. Every Target in `solar-system` then gets the agent through its
bootstrap chart, with its own credential. The dev cluster's `solar` values
enable agent credentials with the in-cluster apiserver URL.

```bash
kubectl --context kind-solar-dev -n solar-system get releasebindings,targetreports
```

To try an unreleased chart or image, change the constructor in
opendefensecloud/ocm, build it there with `make build COMPONENT=solar-agent`,
and point `OCM_AGENT_REPO` at the result. Bump the component version for every
change: `make demo-app` skips the transfer once the ComponentVersion for
`OCM_AGENT_VERSION` exists. The constructor has to reference the chart by its
OCI reference, not `input: helm`: discovery can't use a chart stored as a local
blob, and the wait for the ComponentVersion just times out.
`OCM_AGENT_REPO=ctf::../ocm/transport-archive OCM_AGENT_VERSION=<new version> make demo-app`.

### Watching the Results

On startup the agent resolves its `Target`, which proves three things at once:
the endpoint answers as a solar-apiserver, the credential is accepted, and the
`Target` exists. It then writes its `TargetReport` on each tick.

Each `OCIRepository`/`HelmRelease` pair the bootstrap chart created becomes one entry with a phase:

| Phase         | Means                                                                 |
| ------------- | --------------------------------------------------------------------- |
| `Pending`     | only one half of the pair exists                                      |
| `Progressing` | Flux is still working, or the conditions describe an older generation |
| `Ready`       | both halves report `Ready=True`                                       |
| `Degraded`    | not ready, but Flux will retry                                        |
| `Failed`      | terminal: `Stalled` on the source, or helm remediation out of retries |

## Rebuilding Without Full Setup

After making code changes, rebuild images and reload them:

```bash
make dev-cluster-rebuild
```

This builds and loads Docker images without reinstalling everything.

## Cleaning Up

Remove the development cluster:

```bash
make cleanup-dev-cluster
```

Or delete only the Kind cluster:

```bash
kind delete cluster --name solar-dev
```

## Troubleshooting

### Webhook Not Ready

If you see TLS certificate errors related to webhooks, wait a moment for cert-manager to initialize, then retry:

```bash
kubectl get pods -n cert-manager
kubectl get certificates -n cert-manager
```

### Images Not Loading

Verify images are loaded into Kind:

```bash
kind get images --name solar-dev
```

### LoadBalancers Pending

Another cluster may already own the fixed registry IPs. Check
`docker network inspect solar-kind` and `docker logs solar-kind-cloud-provider`.
Delete the unused cluster with `kind delete cluster --name <name>`; the provider
will release its addresses and retry.
