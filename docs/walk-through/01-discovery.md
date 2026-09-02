# Discovery

## Prerequisites

- SolAr is installed in a dev-cluster. See [Getting Started](../getting-started.md).
- SolAr's dependencies (cert-manager, trust-manager) are installed, and the `zot-discovery` / `zot-deploy` registries are running. `make dev-cluster` sets all of this up for you.

## Register the discovery registry

Discovery is a separate component (`solar-discovery`) that watches one or more
OCI registries and creates `Component` / `ComponentVersion` resources when it
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
  hostname: 10.96.200.10:443
  flavor: zot
  webhookPath: events
  solarSecretRef:
    name: zot-discovery-auth
  targetPullSecretName: regcred
```

```bash
kubectl apply -f registry.yaml
```

> **Why an IP, not a DNS name?** This `hostname` gets stamped verbatim into
> every discovered `ComponentVersion`'s resource references — including
> container image references. Those images are later pulled by a target
> cluster's kubelet/containerd directly on the node, which (unlike pods) does
> not use the cluster's CoreDNS and can't resolve `*.svc.cluster.local`
> names. `zot-discovery`'s dev-cluster Service is pinned to a fixed
> `ClusterIP` specifically so it can be referenced this way. The render/deploy
> registry (`zot-deploy`) doesn't need this treatment: its hostname is only
> ever resolved by Flux's `OCIRepository` fetches, which run from a pod and
> use CoreDNS normally. Kubelet/containerd DNS is host-controlled and independent
> of Kubernetes Service DNS, so a `*.svc.cluster.local` hostname is not guaranteed
> to resolve on the node in any cluster. Use a registry hostname/IP that's routable
> from the nodes, or configure a containerd registry mirror/host alias that maps a
> kubelet-resolvable name to the in-cluster registry.

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

Start a local port-forward for the zot-discovery registry.

```bash
kubectl port-forward -n zot svc/zot-discovery 4443:443 &
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
  - type: rootcerts.config.ocm.software
    rootCertificates:
      - path: ./ca.crt
  - type: credentials.config.ocm.software
    consumers:
      - identity:
          type: OCIRegistry
          scheme: https
          hostname: localhost
          port: 4443
        credentials:
          - type: Credentials
            properties:
              username: admin
              password: admin
  - type: oci.uploader.config.ocm.software
    preferRelativeAccess: true
```

```bash
./bin/go/ocm --config ./ocmconfig transfer ctf ./test/fixtures/ocm-demo-ctf https://localhost:4443/test
```

Take a look at the discovery registry: <https://localhost:4443/explore>. The
component versions as well as the component descriptors were added.

The `ComponentVersion` was discovered by SolAr:

```console
$ kubectl get componentversions -n test
NAME                                 CREATED AT
opendefense-cloud-ocm-demo-v26-4-2   2026-07-24T11:15:24Z

$ kubectl get components -n test
NAME                         CREATED AT
opendefense-cloud-ocm-demo   2026-07-24T11:15:24Z
```
