# Several Helm charts in one component

Fixtures and results for the spike in
[#941](https://github.com/opendefensecloud/solution-arsenal/issues/941): how
should one component carry several Helm charts?

Two charts stand in for the usual pair of an operator and what it manages:

- `charts/widget-operator`: the `Widget` CRD, and a controller whose only job
  is to release the finalizer of a `Widget` that is being deleted.
- `charts/widget-instance`: a `Widget`, which carries that finalizer, and a
  Deployment.

So the instance needs the operator's CRD to install, and the running operator
to be removed.

They are packaged in three ways:

| Directory | Model | SolAr today |
|---|---|---|
| `umbrella/` | **A, Helm sub-charts.** One chart with both as sub-charts, one `helmChart` resource. | deploys it unchanged |
| `chart-set/` | **B, one resource per chart.** Two `helmChart` resources. `chart-set/release/` is the release chart SolAr could render for it, written by hand: one `HelmRelease` per chart, ordered with Flux `dependsOn`. | discovery rejects the component |
| `components/` | **C, one component per chart.** `components/outer.yaml` is what the bootstrap chart could create for two Releases, written by hand: one `HelmRelease` per Release, the second with `dependsOn` on the first. | deploys each component, in no order |

## Running them

On a dev cluster (`make dev-cluster`), from the repository root. `publish.sh`
pushes the charts to the discovery registry and transfers the component.

Model A, through SolAr. The Release needs the Target of the demo, so seed that
first (`make demo-app`), or apply the Registry, RegistryBindings, Target and
bootstrap from `test/fixtures/e2e` as `hack/demo/demo-app.sh` does.

```sh
UMBRELLA_CRDS_DIR=1 test/fixtures/multi-helm/publish.sh umbrella 0.1.0
kubectl create namespace widgets
kubectl label namespace widgets trust=enabled
kubectl apply -n widgets -f test/fixtures/e2e/regcred.yaml
kubectl apply -n solar-system -f test/fixtures/multi-helm/umbrella/release.yaml
```

Model B, with Flux alone, as SolAr cannot render it:

```sh
test/fixtures/multi-helm/publish.sh chart-set 0.1.0
kubectl create namespace widgets-set
kubectl label namespace widgets-set trust=enabled
kubectl apply -n widgets-set -f test/fixtures/e2e/regcred.yaml -f test/fixtures/multi-helm/chart-set/outer.yaml
```

Model C, with Flux alone as well. It uses the charts and the release chart
that `publish.sh chart-set` pushes, each release cut down to one chart:

```sh
kubectl create namespace widgets-c
kubectl label namespace widgets-c trust=enabled
kubectl apply -n widgets-c -f test/fixtures/e2e/regcred.yaml -f test/fixtures/multi-helm/components/outer.yaml
```

Single fields of the release chart are set through the outer `HelmRelease`,
for example a namespace of its own for the instance:

```sh
kubectl -n widgets-set patch helmrelease widgets-set --type merge \
  -p '{"spec":{"values":{"charts":{"instance":{"targetNamespace":"widgets-apps"}}}}}'
```

A version can be published once. Pass a higher one after changing a chart:
the outer `OCIRepository` objects of B and C take the highest version in the
registry.

Run one model at a time and remove it before the next, including the `Widget`
CRD, which model A leaves behind. The operator watches all namespaces, so one
left running releases the finalizers of another model's `Widget` and hides a
stuck removal.

After a removal that got stuck, the `Widget` still carries its finalizer and
holds back the CRD. Release it, then delete the CRD:

```sh
kubectl get widgets -A
kubectl -n <namespace> patch widget <name> --type merge -p '{"metadata":{"finalizers":null}}'
kubectl delete crd widgets.multi-helm.solar.opendefense.cloud
```

## Results

Run on 2026-11-18 against Kubernetes 1.37.0 and Flux helm-controller 1.7.1,
with SolAr from `main` at `d853aaa4`.

### Install

| | A, umbrella | B, chart set |
|---|---|---|
| CRD as a template of the operator chart (as CloudNativePG and cert-manager ship it) | **Fails.** Helm rejects the release before installing anything: `no matches for kind "Widget" ... ensure CRDs are installed first`. | Works. The instance waits with `dependency ... is not ready`, then installs. |
| CRD in the umbrella's `crds/` directory, switched off in the sub-chart (`UMBRELLA_CRDS_DIR=1`) | Works, in about 20 s. | not needed |
| Time until everything is Ready | about 20 s | about 40 s: helm-controller rechecks a dependency every 30 s |

Model A therefore needs the author to move every CRD another sub-chart uses
out of its chart. A chart that offers no switch for its CRDs cannot be used
this way without a patch.

### Failure and upgrade

- **Both models report a failure upwards.** The `HelmRelease` of the Release,
  and the bootstrap one above it, stay not Ready and fail with
  `timeout waiting for: [HelmRelease/... status: 'InProgress']` after 5
  minutes.
- **SolAr did not show it.** While Flux reported the failed install, neither
  the Release nor the Target carried a failing condition. The dev cluster runs
  no solar-agent, which is what reports deployment status.
- **In B a failing upgrade held back the dependent.** With a broken operator
  and a changed instance applied together, the instance was held back until
  the timeout, the release chart was rolled back, and the running pods were
  never replaced.
- **That is not ordering by version.** The charts are referenced through an
  `OCIRepository`, so a new chart version does not change a `HelmRelease`,
  and `dependsOn` accepts a dependency that is still Ready at its old
  version. An instance can then be upgraded before its operator. Not
  observed and not provoked here. Flux has `readyExpr` for this case.
- In A an upgrade is one Helm operation over all sub-charts, in Helm's order by
  kind. Not ordered by chart.

### Removal

| | A, umbrella | B, chart set |
|---|---|---|
| Delete the release | **Stuck.** Helm deletes the `Widget` and the operator in one pass. The operator is gone before it releases the finalizer. | **Stuck.** Both `HelmRelease`s are deleted at once: `dependsOn` does not order removal. |
| After the 5-minute timeout | Uninstall fails, the `HelmRelease` objects go away regardless, the `Widget` stays in Terminating. | The same, and the CRD stays in Terminating behind the `Widget`. |
| CRD | never removed: Helm does not delete what `crds/` installed | removed with the operator chart |
| Remove the instance first, then the rest | not possible: one release | **Clean, in 12 s.** Nothing left behind, CRD included. |

Neither model removes an operator and its instance cleanly as it stands. In B
the order is known and SolAr could apply it in reverse. In A it is inside one
Helm release, out of SolAr's reach.

### Two Releases in order (C)

Run on 2026-11-19 with the charts at 0.5.0.

- **`dependsOn` orders Releases too.** The instance's `HelmRelease` reported
  `dependency 'widgets-c/operator' is not ready` until the operator's, and the
  chart inside it, were ready. Everything was Ready after about 45 s.
- **Removal in reverse order is clean.** Instance first, then operator: 8 s,
  nothing left behind, CRD included.
- Removing both at once was not run. It is the case that gets stuck in B.

### Values, images, namespaces

| | A, umbrella | B, chart set |
|---|---|---|
| Values | one document, keyed by sub-chart name; a pull secret is set once per sub-chart | per chart |
| Image re-mapping | one values template for all sub-charts, kept by hand under each sub-chart's key. Worked here. A wrong key is no error: that sub-chart keeps its upstream image. | needs a values template per chart. Not built: nothing renders the release chart. |
| Namespace | one for all sub-charts | per chart (`targetNamespace`). Worked, the namespace was created. |
| Chart version | must change with every component version: the transfer publishes the chart under its own tag | per chart |

Moving a chart that is already installed to another namespace (B) installed it
there and left its objects in the old namespace behind. Seen once, not
investigated.

### What SolAr does with a chart-set component today

Discovery logs `no handler found for component version event` on every scan,
and the component never reaches the catalog: `pkg/discovery/handler/handler.go`
accepts a component only with exactly one `helmChart` resource.

### Several components and archives

Checked on 2026-11-19 with the OCM v2 CLI 0.21.0 and a throwaway constructor
file, not with the fixtures:

- One constructor file with two components gives one archive with both.
- A `componentReferences` entry records the digest of the component it names.
- Transferring the referencing component to another archive leaves the
  referenced one behind, without an error. With `--recursive` both arrive.

### Namespaces of real charts

Checked on 2026-11-19 by rendering each chart with its defaults into a
neutral namespace, `helm template <name> <chart> --version <v> -n some-ns`.
Nothing was installed.

| Chart | Objects outside `some-ns` |
|---|---|
| Tetragon 1.8.0 | none |
| Istio `istiod` 1.30.8 | 4 in `istio-system`, among them a ServiceAccount and a Role |
| cert-manager v1.21.3 | 4 in `kube-system`, the leader election Roles and RoleBindings |

external-secrets 2.15.0 with `webhook.certManager.enabled=true` renders a
`cert-manager.io/v1` `Certificate`.

### Packaging

With the OCM v2 CLI a chart added by `input: helm` stays a blob inside the
component, which Flux cannot pull. Both constructors therefore reference their
charts by OCI reference, and `publish.sh` pushes the charts first.
