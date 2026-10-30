# Agent Registration

Every `Target` gets its own `solar-agent`. The agent reports the state of the
target cluster back to SolAr by updating one `TargetReport`. This page explains
how the agent gets its credential and what you do by hand.

An operator must set up the agent once per SolAr installation. See [Setting up
the solar-agent](../operator-manual/solar-agent.md). If that is done, you only
create the Target and run the first-contact step below.

## What happens when you create a Target

| Step | Owner |
| ---- | ----- |
| Create the `Target` | you |
| Create one agent `Release` (annotated `target-credential`) and one `Profile` with an empty selector, once per install | operator |
| Create a ServiceAccount, Role, RoleBinding and an empty `TargetReport`; mint a token into a Secret | solar-controller-manager |
| Render the agent release with the kubeconfig and the token | solar-renderer |
| Pull and install, like every other bound Release | Flux on the target |
| Update its own `TargetReport` | the agent |

The Profile has an empty `targetSelector`, so it matches every Target in the
namespace. The agent Release is bound to your new Target without further steps.
The credential only allows the agent to read its own `Target` and to read and
update its own `TargetReport`.

## First contact (manual)

SolAr cannot reach into the target cluster. Flux on the target cluster must be
pointed at the Target's render registry once. After that, Flux installs the
agent and every other bound Release on its own.

On the target cluster, create a pull Secret for the render registry, an
`OCIRepository` and a `HelmRelease`. The following objects are taken from the
[walk-through](../walk-through/03-bootstrap.md#create-a-helm-release-for-the-bootstrap-chart).
Replace the registry host, the namespace and the Target name with your values.
The URL has the form `oci://<render-registry>/<namespace>/bootstrap-<target>`.

```yaml
# regcred.yaml
apiVersion: v1
kind: Secret
metadata:
  name: regcred
  namespace: test
type: kubernetes.io/dockerconfigjson
stringData:
  .dockerconfigjson: |
    {
      "auths": {
        "zot-deploy.zot.svc.cluster.local": {
          "username":"user",
          "password":"user",
          "auth":"dXNlcjp1c2Vy"
        }
      }
    }
```

```yaml
# helmrelease.yaml
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: solar-bootstrap
  namespace: test
spec:
  interval: 5m0s
  url: oci://zot-deploy.zot.svc.cluster.local/test/bootstrap-cluster-1
  layerSelector:
    mediaType: "application/vnd.cncf.helm.chart.content.v1.tar+gzip"
    operation: copy
  ref:
    semver: ">=0.0.0"
  secretRef:
    name: regcred
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: solar-bootstrap
  namespace: test
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
kubectl apply -n test -f regcred.yaml
kubectl apply -n test -f helmrelease.yaml
```

The pull credentials in `regcred` should only reach this Target's namespace in
the registry. See the [security notes](../operator-manual/solar-agent.md#security).

## Check that it worked

On the SolAr cluster, check the credential condition on the Target:

```bash
kubectl get target cluster-1 -n test -o jsonpath='{.status.conditions}'
```

Look for `AgentCredentialReady`:

| Status | Reason | Meaning |
| ------ | ------ | ------- |
| `True` | `Issued` | A token exists. The message names the Secret and its expiry. |
| `False` | `Disabled` | The operator has not set `controller.agent.apiserverURL`. |
| `False` | `Revoked` | The Target carries `solar.opendefense.cloud/agent-credential: revoked`. |
| `False` | `MintFailed` | The token could not be created. The message has the error. The controller retries. |

Once Flux has installed the agent, the report updates:

```bash
kubectl get targetreport cluster-1 -n test -o jsonpath='{.spec.lastReportTime}'
```

`lastReportTime` is empty until the first report and then advances with every
report. If it stays empty, check the agent pod logs on the target cluster.

To rotate or revoke a credential, see the [runbook](../operator-manual/solar-agent.md#runbook).
