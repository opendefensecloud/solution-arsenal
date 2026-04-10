# Releases

## Create a release of the ComponentVersion

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
```

```bash
kubectl apply -n test -f ./release.yaml
```

The release was created

```bash
kubectl get release -n test
```

```bash
kubectl describe release -n test
```

A rendertask was also created to create a helm-chart which rolls out the ocm-demo component.

```bash
kubectl get rendertasks
```

The rendered chart was pushed to the zot-deploy registry:

```bash
kubectl port-forward -n zot svc/zot-deploy 4444:443 &
```

<https://localhost:4444/explore>

You can template the release chart to inspect the rendered result:

```bash
SSL_CERT_FILE=./ca.crt helm template foo \
    --username admin --password admin \
    oci://localhost:4444/test/release-test-opendefense-cloud-ocm-demo-v26-4-0-release:v0.0.0
```
