# Discovery

## Prerequisites

- SolAr is installed in a dev-cluster. See [Getting Started](../getting-started.md).
- SolAr's dependencies (cert-manager, trust-manager) are installed, and the `zot-discovery` / `zot-deploy` registries are running. `make dev-cluster` sets all of this up for you.

## Register the discovery registry

Discovery is a separate component (`solar-discovery`) that watches one or more
OCI registries and creates `ComponentVersion` resources when it
finds OCM packages. You configure a `Registry` for solar-discovery to watch, and
deploy solar-discovery itself pointed at the namespace that `Registry` lives in.

The following manifest registers `zot-discovery` for webhook-based discovery
in the `test` namespace.

```yaml
# registry.yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: test
  labels:
    trust: enabled
---
apiVersion: v1
kind: Secret
metadata:
  name: zot-discovery-auth
  namespace: test
type: Opaque
stringData:
  username: admin
  password: admin
---
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Registry
metadata:
  name: zot-webhook
  namespace: test
spec:
  hostname: 172.30.255.200:443
  flavor: zot
  webhookPath: events
  solarSecretRef:
    name: zot-discovery-auth
  targetPullSecretName: regcred
```

```bash
kubectl apply -f registry.yaml
```

> The dev cluster pins the discovery LoadBalancer to `172.30.255.200`.
> Uploads and resource consumers use this same address. It must be reachable
> from your host, pods and target nodes: kubelet/containerd pulls images on the
> node and cannot rely on cluster Service DNS.

```console
$ kubectl get registries -n test
NAME          CREATED AT
zot-webhook   2026-07-24T11:14:18Z
```

`flavor: zot` and `webhookPath: events` enable webhook mode: `zot-discovery`
pushes an event to solar-discovery whenever an image is pushed or deleted, so
new component versions show up immediately. See [SolAr
Discovery](../user-guide/discovery.md) for scan mode and the other available
options.

## Point solar-discovery at this namespace

`make dev-cluster` already installs a `solar-discovery` Helm release into
`solar-system`. Its `namespace` value controls which namespace it _watches_
for `Registry` resources — independent of the namespace the Deployment itself
runs in. Reconfigure that release to watch `test`, rather than installing a
second release: solar-discovery's `ClusterRole` / `ClusterRoleBinding` are
cluster-scoped and named after the release, so installing a second
`solar-discovery` release into another namespace fails with an ownership
conflict.

```bash
helm upgrade solar-discovery oci://ghcr.io/opendefensecloud/charts/solar-discovery \
  --namespace solar-system \
  --reuse-values \
  --set namespace=test
```

```bash
kubectl rollout status deployment/solar-discovery -n solar-system
```

The dev cluster's `zot-discovery` registry already has its webhook sink
pointed at `solar-discovery.solar-system.svc.cluster.local` — since the
Deployment stays in `solar-system`, no further wiring is needed.

## Transfer example component version

Read the discovery registry's LoadBalancer address.

```bash
DISCOVERY_IP=$(kubectl -n zot get svc zot-discovery -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```

Prepare the CA certificate of zot and the `ocmconfig` for the `ocm transfer`
command.

```bash
kubectl get secrets -n cert-manager selfsigned-ca-secret -oyaml \
   | yq -r '.data."tls.crt" | @base64d' > ca.crt
```

```yaml
# ocmconfig
type: generic.config.ocm.software/v1
configurations:
  - type: credentials.config.ocm.software/v1
    consumers:
      - identity:
          type: OCIRegistry
          hostname: 172.30.255.200
          port: "443"
        credentials:
          - type: Credentials/v1
            properties:
              username: admin
              password: admin
```

OCM v2 has no `rootcerts` config type, so the CA is supplied through
`SSL_CERT_FILE`. The uploader configuration in
`test/fixtures/ocm-uploaders.yaml` publishes the chart and the image as tagged
OCI artifacts in the target registry; without it they would stay addressable
only by digest inside the component's own repository. `--config` is repeatable,
so the uploader and credential documents are merged.

```bash
SSL_CERT_FILE=./ca.crt ./bin/go/ocm2 \
  --config ./test/fixtures/ocm-uploaders.yaml --config ./ocmconfig transfer cv \
  'ctf::./test/fixtures/ocm-demo-ctf//opendefense.cloud/ocm-demo:0.1.0' \
  "https://${DISCOVERY_IP}:443/test"
```

Open `https://${DISCOVERY_IP}:443/explore` to see the uploaded component versions
and component descriptors.

The `ComponentVersion` was discovered by SolAr:

```console
$ kubectl get componentversions -n test
NAME                                 CREATED AT
opendefense-cloud-ocm-demo-0-1-0     2026-07-24T11:15:24Z
```
