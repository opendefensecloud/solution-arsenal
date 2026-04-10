# Releases

`Releases` allow the user to add parameters to a `ComponentVersion`. They can
also be referenced by `Profiles` or directly by `Targets`. This abstraction
enables building complex deployment scenarios with multiple applications and
multiple clusters.

## Create a `Release` of the `ComponentVersion`

Create a Release resource that references our discovered `ComponentVersion`:

```yaml
# release.yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Release
metadata:
  name: test-opendefense-cloud-ocm-demo-v26-4-0-release
  namespace: test
spec:
  componentVersionRef:
    name: opendefense-cloud-ocm-demo-v26-4-0
  values:
    replicaCount: 3
```

```bash
kubectl apply -n test -f ./release.yaml
```

The Release was created and can be inspected with kubectl:

```console
$ kubectl get release -n test
NAME                                              CREATED AT
test-opendefense-cloud-ocm-demo-v26-4-0-release   2026-04-10T11:20:36Z
```

A `RenderTask` was also created to create a helm-chart. This helm chart will
roll out the `ComponentVersion` with our parameterization.

```console
$ kubectl get rendertasks
NAME                                                     CREATED AT
test-test-opendefense-cloud-ocm-demo-v26-4-0-release-0   2026-04-10T11:20:36Z
```

The `RenderTask` pushed the created chart to the zot-deploy registry.

Let's create a port-forward to the cluster to look inside the zot registry:

```bash
kubectl port-forward -n zot svc/zot-deploy 4444:443 &
```

The zot UI can now be accessed at
[https://localhost:4444](https://localhost:4444/explore)

You can template the release chart with the helm CLI to inspect the rendered
result:

```bash
SSL_CERT_FILE=./ca.crt helm template foo \
    --username admin --password admin \
    oci://localhost:4444/test/release-test-opendefense-cloud-ocm-demo-v26-4-0-release:v0.0.0
```
