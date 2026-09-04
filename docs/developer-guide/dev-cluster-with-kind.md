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

The Zot registries use ClusterIP services. Use `kubectl port-forward` to access them:

```bash
# Terminal 1: Forward zot-discovery
kubectl -n zot port-forward svc/zot-discovery 4443:443

# Terminal 2: Forward zot-deploy
kubectl -n zot port-forward svc/zot-deploy 4444:443
```

Then access at:

- **zot-discovery**: `https://localhost:4443`
- **zot-deploy**: `https://localhost:4444`

### Pushing Images

Tag and push images to the local registries:

```bash
# Push to zot-discovery
docker tag localhost/local/solar-discovery-worker:dev.* \
    localhost:4443/solar-discovery-worker:local
docker push localhost:4443/solar-discovery-worker:local

# Push to zot-deploy
docker tag localhost/local/solar-apiserver:dev.* \
    localhost:4444/solar-apiserver:local
docker push localhost:4444/solar-apiserver:local
```

### Pulling Images

Configure your local Kubernetes cluster to pull from the local registries:

```bash
# Create image pull secret if needed
kubectl create secret docker-registry zot-creds \
    --docker-server=localhost:4443 \
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

1. Wait for zot-discovery to be ready
2. Start a port-forward to zot-discovery
3. Transfer the ocm-demo component via OCM (using `test/fixtures/e2e/ocmconfig`,
   which trusts the cluster CA)
4. Clean up the port-forward (always, via a trap)

Since `make dev-cluster` deploys solar-discovery in scan mode with the discovery
Zot already registered, you do not apply a `Registry` yourself. A few seconds
after the transfer the discovery worker scans the registry and creates the
`Component` and `ComponentVersion` in the `solar-system` namespace:

```bash
kubectl --context kind-solar-dev -n solar-system get components,componentversions
```

### Environment Variables

| Variable           | Default                      | Description             |
| --                 | --                           | --                      |
| `KIND_CLUSTER_DEV` | `solar-dev`                  | Kind cluster name       |
| `KUBECTL`          | `kubectl`                    | Kubernetes CLI          |
| `OCM`              | `ocm`                        | OCM CLI path. The Makefile provisions it into `bin/go/ocm`, which is not on `PATH`, so for standalone use pass `OCM=./bin/go/ocm` |
| `OCM_CONFIG`       | `./test/fixtures/e2e/ocmconfig` | ocm config file (needs the rootcerts block that trusts the cluster CA) |
| `OCM_DEMO_DIR`     | `test/fixtures/ocm-demo-ctf` | ocm-demo CTF location   |
| `LOCAL_PORT`       | `4443`                       | local port for the zot-discovery port-forward |

Example:

```bash
OCM=./bin/go/ocm KIND_CLUSTER_DEV=my-cluster ./test/fixtures/setup-discovery.sh
```

## Setting Up Release for Testing

The `test/fixtures/setup-release.sh` script creates Component and Release resources to test if RenderTasks are created correctly and can push to the zot-deploy registry.

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
kubectl get components,componentversions,releases,jobs,pods -n my-namespace -w
```

The flow is:

1. **Component** and **ComponentVersion** are created in the namespace
2. **Release** is created in the namespace
3. The rendertask_controller creates a **Job** in the same namespace
4. The Job spawns a **Pod** that renders the release and pushes it to the zot-deploy registry

## Setting Up Solar Agent for Testing

The `test/fixtures/setup-agent.sh` script prepares a `Target` and a scoped credential so `solar-agent` can be run
against the dev cluster.

### When to Use It

Use this script when you want to:

- Watch the agent roll Flux `OCIRepository`/`HelmRelease` pairs up into release status
- Check what the agent reports for a release you have deliberately broken
- Verify the agent's RBAC is sufficient, and no wider than it needs to be

### How the Flow Works

Per [ADR 018](adrs/018-Solar-Agent-Architecture.md), a user creates the `Target` and SolAr renders the agent for
it. The agent creates nothing and installs nothing: it reads the Flux objects on its own cluster and reports what
it finds. The script therefore does the user's half of that flow, and leaves you to start the agent by hand.

The dev cluster plays both roles at once. It is the solar cluster the agent reports to and the target cluster the
agent reads from, so the two RBAC fixtures the script applies are separate on purpose:

| Fixture                                   | Cluster it belongs to | Grants                                                   |
| ----------------------------------------- | --------------------- | -------------------------------------------------------- |
| `test/fixtures/e2e/agent-rbac.yaml`       | solar                 | `get`/`list` on `targets` in one namespace               |
| `test/fixtures/e2e/agent-local-rbac.yaml` | target                | `list` on nodes, pods, `ocirepositories`, `helmreleases` |

### Prerequisites

None to start the agent. But to have it report anything, the cluster needs Flux pairs carrying the
`solar.opendefense.cloud/release` label, which is what the agent selects on. Only the rendered bootstrap chart
(`pkg/renderer/template/bootstrap/templates/release.yaml`) sets that label:

```bash
./test/fixtures/setup-discovery.sh   # transfer the ocm-demo component
./test/fixtures/setup-release.sh     # render it and push to zot-deploy
./test/fixtures/setup-bootstrap.sh   # let Flux pull the rendered chart
```

Note that `test/fixtures/e2e/bootstrap-ocirepository.yaml` and `bootstrap-helmrelease.yaml`, applied by
`setup-bootstrap.sh`, are static fixtures with no labels on them. The agent will not pick those up. Label them by
hand to see a pair reported without going through a full render.

### Running the Script

```bash
./test/fixtures/setup-agent.sh
```

This will:

1. Create the namespace(s) if they don't exist
2. Apply both RBAC fixtures from the table above
3. Ensure the `deploy-registry` `Registry` exists, so `RegistryResolved` succeeds rather than failing `NotFound`
4. Apply a `ReferenceGrant`, if the Target and Registry namespaces differ
5. Create the `Target`
6. Mint a token for the agent's ServiceAccount and write it as a kubeconfig
7. Print the `go run ./cmd/solar-agent ...` command to start the agent with

The kubeconfig stands in for the OAuth client credential ADR 018 specifies. Once the issuer exists, SolAr renders
that credential into the agent's manifests and this step disappears.

### Watching the Results

Run the printed command. On startup the agent resolves its `Target`, which proves three things at once: the
endpoint answers as a solar-apiserver, the credential is accepted, and the `Target` exists. It then reports on each
tick, and the report is logged rather than sent anywhere.

Each `OCIRepository`/`HelmRelease` pair the bootstrap chart created becomes one entry with a phase:

| Phase         | Means                                                                 |
| ------------- | --------------------------------------------------------------------- |
| `Pending`     | only one half of the pair exists                                      |
| `Progressing` | Flux is still working, or the conditions describe an older generation |
| `Ready`       | both halves report `Ready=True`                                       |
| `Degraded`    | not ready, but Flux will retry                                        |
| `Failed`      | terminal: `Stalled` on the source, or helm remediation out of retries |

The script also prints a second command pointing at a `Target` that does not exist. Running it once is the quickest
way to see the startup check fail on purpose.

### Exercising the Cross-Namespace Path

By default the `Target` lands in `solar-system` alongside the `Registry`, so no `ReferenceGrant` is involved. To
put them in different namespaces and pull that path in:

```bash
NAMESPACE=tenant-demo REGISTRY_NAMESPACE=solar-system TARGET_NAME=agent-cross-ns \
  ./test/fixtures/setup-agent.sh
```

### Environment Variables

| Variable             | Default                       | Description                                                   |
| -------------------- | ----------------------------- | ------------------------------------------------------------- |
| `KIND_CLUSTER_DEV`   | `solar-dev`                   | Kind cluster name                                             |
| `KUBECTL`            | `kubectl`                     | Kubernetes CLI                                                |
| `NAMESPACE`          | `solar-system`                | Namespace the Target is created in                            |
| `TARGET_NAME`        | `cluster-1`                   | Name of the Target the agent reports for                      |
| `RENDER_REGISTRY`    | `deploy-registry`             | Name of the Registry the Target references                    |
| `REGISTRY_NAMESPACE` | same as `NAMESPACE`           | Namespace the Registry lives in; differs to pull in the grant |
| `OUT_KUBECONFIG`     | `/tmp/solar-agent.kubeconfig` | Where to write the agent kubeconfig                           |

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

### Port Conflicts

If ports 4443 or 4444 are in use, modify the service port in the respective values files under `test/fixtures/`.
