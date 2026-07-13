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

This should be take care of if you use the Makefile.

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

## What Gets Installed

| Component     | Namespace    | Description                      |
| ------------- | ------------ | -------------------------------- |
| cert-manager  | cert-manager | TLS certificate management       |
| trust-manager | cert-manager | Trust bundle management          |
| zot-discovery | zot          | OCI registry for discovery       |
| zot-deploy    | zot          | OCI registry for deployment      |
| solar         | solar-system | SolAr API server and controllers |

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
3. Transfer the ocm-demo component via OCM
4. Clean up the port-forward

### Environment Variables

| Variable           | Default                      | Description           |
| ------------------ | ---------------------------- | --------------------- |
| `KIND_CLUSTER_DEV` | `solar-dev`                  | Kind cluster name     |
| `KUBECTL`          | `kubectl`                    | Kubernetes CLI        |
| `OCM`              | `ocm`                        | OCM CLI path          |
| `OCM_DEMO_DIR`     | `test/fixtures/ocm-demo-ctf` | ocm-demo CTF location |

Example:

```bash
KIND_CLUSTER_DEV=my-cluster ./test/fixtures/setup-discovery.sh
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

## Setting Up Solar Agent Workflows for Testing

`solar-agent` supports two ways a `Target` and its agent come to exist together; see
[ADR 014](adrs/014-Solar-Agent-Architecture.md) for the reasoning behind both. Each has its own setup script.

### Workflow A: Agent Self-Registration

The agent creates its own `Target` on startup instead of one having to already exist.

#### Running the Script

```bash
./test/fixtures/setup-agent-self-register.sh
```

This will:

1. Create the namespace(s) if they don't exist
2. Apply a ServiceAccount/Role/RoleBinding scoped to `get`/`list`/`create` on `targets` in the target namespace only
3. Ensure a real `Registry` (`deploy-registry`, pointing at zot-deploy) exists in the registry namespace, so
   `RegistryResolved` actually succeeds instead of failing with `NotFound`
4. If the target and registry namespaces differ, apply a `ReferenceGrant` permitting the target namespace's
   `Target`s to reference `Registry`s in the registry namespace
5. Mint a token and write a bootstrap kubeconfig
6. Print the `go run ./cmd/solar-agent ...` command to run with it

Run the printed command, then verify:

```bash
kubectl get target agent-self-registered -n solar-system -o yaml
```

By default the `Target` self-registers directly into `solar-system` (same namespace as the `Registry`, so no
`ReferenceGrant` is needed. To exercise the cross-namespace path instead, point the target namespace elsewhere
and the registry namespace at `solar-system`:

```bash
NAMESPACE=tenant-demo REGISTRY_NAMESPACE=solar-system TARGET_NAME=agent-cross-ns \
  ./test/fixtures/setup-agent-self-register.sh
```

#### Environment Variables

| Variable             | Default                                 | Description                                                                            |
| -------------------- | --------------------------------------- | -------------------------------------------------------------------------------------- |
| `KIND_CLUSTER_DEV`   | `solar-dev`                             | Kind cluster name                                                                      |
| `KUBECTL`            | `kubectl`                               | Kubernetes CLI                                                                         |
| `NAMESPACE`          | `solar-system`                          | Namespace the Target self-registers into                                               |
| `TARGET_NAME`        | `agent-self-registered`                 | Name the agent registers itself under                                                  |
| `RENDER_REGISTRY`    | `deploy-registry`                       | Name of the Registry to reference                                                      |
| `REGISTRY_NAMESPACE` | same as `NAMESPACE`                     | Namespace the Registry lives in; set differently to exercise the `ReferenceGrant` path |
| `OUT_KUBECONFIG`     | `/tmp/solar-agent-bootstrap.kubeconfig` | Where to write the bootstrap kubeconfig                                                |

### Workflow B: Solar-Initiated Remote Install

The `test/fixtures/setup-agent-remote-install.sh` script creates a `Target` with `agentAccessSecretRef` set, so
solar-controller-manager installs the agent itself instead of waiting for a manual deploy.

#### Running the Script

```bash
./test/fixtures/setup-agent-remote-install.sh
```

This will apply:

- `test/fixtures/e2e/agent-remote-install-rbac.yaml` -- ServiceAccount + ClusterRole for the remote installer
- `test/fixtures/e2e/agent-remote-install-target.yaml` -- a Target with `agentAccessSecretRef` set

and imperatively create a ClusterRoleBinding plus a `kubeconfig` Secret, since both need the namespace filled in at
apply time.

This demo is self-referential: the "remote" cluster the installer targets is the same kind-solar-dev cluster
solar-apiserver runs in (via the in-cluster `https://kubernetes.default.svc` address), since a real second target
cluster isn't part of the dev-cluster setup. Against a real target cluster, the Secret would hold that cluster's own
kubeconfig instead.

#### Watching the Results

```bash
kubectl get target agent-remote-install -n tenant-demo -w
```

The flow is:

1. `TargetAgentInstallerReconciler` sees `agentAccessSecretRef` set and `AgentInstalled` not yet `True`
2. It reads the kubeconfig Secret and calls the current `AgentInstaller` (`MarkerInstaller` -- a placeholder for the
   real `helm upgrade --install` of the `solar-agent` chart, which doesn't exist yet)
3. `MarkerInstaller` creates a marker in the target cluster:
   ```bash
   kubectl get configmap solar-agent-installed -n solar-system -o yaml
   ```
4. `AgentInstalled` flips to `True` on the `Target`

#### Environment Variables

| Variable           | Default       | Description       |
| ------------------ | ------------- | ----------------- |
| `KIND_CLUSTER_DEV` | `solar-dev`   | Kind cluster name |
| `KUBECTL`          | `kubectl`     | Kubernetes CLI    |
| `NAMESPACE`        | `tenant-demo` | Tenant namespace  |

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
