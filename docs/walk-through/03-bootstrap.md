# Bootstrap

## Register the local cluster as a target

```yaml
# target.yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: Target
metadata:
  name: cluster-1
  namespace: test
spec:
  releases:
    test-release:
      name: test-opendefense-cloud-ocm-demo-v26-4-0-release
  userdata:
    foo: bar
    environment: dev
```

```bash
kubectl apply -n test -f target.yaml
```

```bash
kubectl describe bootstrap -n test
```

```bash
kubectl get rendertasks
```

## Create a helm release for the bootstrap chart

```yaml
# helmrelease.yaml
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: solar-bootstrap
spec:
  interval: 5m0s
  url: oci://zot-deploy.zot.svc.cluster.local/test/bootstrap-cluster-1
  layerSelector:
    mediaType: "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
    operation: copy
  ref:
    semver: "^0.0.0"
  secretRef:
    name: regcred
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: solar-bootstrap
spec:
  interval: 10m
  chartRef:
    kind: OCIRepository
    name: solar-bootstrap
  install:
    remediation:
      retries: 3
  upgrade:
    remediation:
      retries: 3
  test:
    enable: true
  driftDetection:
    mode: enabled
  values:
    userdata: {}
```

```bash
kubectl apply -f helmrelease.yaml
```

## Demo app nginx got deployed

```bash
kubectl get pod
```
