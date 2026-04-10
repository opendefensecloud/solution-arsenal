# Discovery

## Prerequisites

- SOLAR is installed in a dev-cluster. See [Getting Started](../getting-started.md).
- SOLAR's dependencies (cert-manager, trust-manager) are installed.
- zot for discovery is setup

## Setup discovery worker

In order to discover ocm packages and make them available to SOLAR a discovery
resource needs to be created. The discovery resource will control a pod running
the discovery-worker configured with a webhook configuration for zot.

The following manifest sets up discovery in the `test` namespace.

```yaml
# discovery.yaml
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
kind: Discovery
metadata:
  name: zot-webhook
  namespace: test
spec:
  registry:
    endpoint: zot-discovery.zot.svc.cluster.local:443
    secretRef:
      name: zot-discovery-auth
    caConfigMapRef:
      name: root-bundle
  webhook:
    flavor: zot
    path: events
```

```bash
kubectl apply -f discovery.yaml
```

```console
$ kubectl get discoveries,svc,pod -n test
NAME                                            CREATED AT
discovery.solar.opendefense.cloud/zot-webhook   2026-04-10T11:14:18Z

NAME                            TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
service/discovery-zot-webhook   ClusterIP   10.96.102.128   <none>        8080/TCP   1m

NAME                        READY   STATUS    RESTARTS   AGE
pod/discovery-zot-webhook   1/1     Running   0          1m
```

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
SSL_CERT_FILE=./ca.crt ./bin/ocm --config ./ocmconfig transfer ctf ./test/fixtures/ocm-demo-ctf https://localhost:4443/test
```

Take a look at the discovery registry: <https://localhost:4443/explore>. The
component versions as well as the component descriptors were added.

The `ComponentVersion` was discovered by SOLAR:

```console
$ kubectl get componentversions -n test
NAME                                 CREATED AT
opendefense-cloud-ocm-demo-v26-4-0   2026-04-10T11:15:24Z

$ kubectl get components -n test
NAME                         CREATED AT
opendefense-cloud-ocm-demo   2026-04-10T11:15:24Z
```
