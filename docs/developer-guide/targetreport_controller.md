# TargetReport Controller

## Overview

The TargetReport controller turns the `TargetReport` a solar-agent writes into
conditions on that agent's `Target`, and the Target's `status.resolvedBindings`
into status on each `ReleaseBinding`. The report stays the agent's raw snapshot;
everything SolAr derives from it lives on the Target and its bindings, so the UI
and other controllers read conditions only.

Report and Target share name and namespace. The report is written only by the
agent, and pre-created empty by the [AgentCredential
controller](agentcredential_controller.md).

```mermaid
flowchart LR
    Agent[solar-agent] -->|updates| Report[TargetReport]
    Report --> Ctrl[TargetReport Controller]
    Ctrl -->|PreflightReady, AgentReporting| Target[Target.status]
    Target -->|resolvedBindings| Ctrl
    Ctrl -->|Deployed| RB[ReleaseBinding.status]
```

## Report states

Each reconcile reads the Target (uncached, see below) and the report, and
classifies the report first:

| State | When |
| ----- | ---- |
| Missing | The Target has no report. No agent is expected. |
| NoReport | The report exists, but `spec.lastReportTime` is empty: the agent never wrote it. |
| Stale | The controller hasn't seen `spec.lastReportTime` change for longer than `--agent-report-stale-after`. |
| Incomplete | The report is fresh, but `spec.releasesError` is set: the agent could not read its Flux objects. `spec.releases` then holds the last successful read. |
| Fresh | The report is fresh and complete. |

A stale report is stale whether or not it carries an error.

## Conditions on the Target

| Condition | Status | Reason | When |
| --------- | ------ | ------ | ---- |
| `PreflightReady` | | | Aggregate of `spec.preflight.checks`, see below |
| `AgentReporting` | `True` | `Reporting` | Fresh or Incomplete |
| `AgentReporting` | `Unknown` | `NoReport` | NoReport |
| `AgentReporting` | `False` | `Stale` | Stale |

A Target without a report gets neither condition. If the report is deleted, both
are removed. With agent credentials enabled the AgentCredential controller
re-creates a missing report, so a deleted report only stays gone while the
Target's credentials are revoked or disabled.

### Preflight

`PreflightReady` rolls up the agent's checks. Per-check results stay in the
report: a snapshot can't know their transitions.

| Status | Reason | When |
| ------ | ------ | ---- |
| `False` | `ChecksErroredAndFailed` | At least one check failed and one could not be evaluated |
| `False` | `ChecksFailed` | At least one check failed |
| `Unknown` | `ChecksErrored` | At least one check could not be evaluated |
| `Unknown` | `NoChecks` | The report has no checks, as the pre-created report |
| `True` | `AllChecksPassed` | Every check passed |

Check reasons are constants in `api/solar/v1alpha1` (`CheckReasonPresent`,
`CheckReasonMissing`, `CheckReasonUnavailable`), as are `PreflightReady` and its
reasons (`ConditionTypePreflightReady`, `ReasonChecksFailed` and so on).

### Liveness

The agent pushes its report when something changed, and at least every
`--max-report-age` (default 10 intervals of 30s, so 5m) as a heartbeat.
`--agent-report-stale-after` (chart value `controller.agent.reportStaleAfter`,
default 10m) is twice that, so one missed heartbeat doesn't flap the Target.
The heartbeat is 10 times the agent's `--interval` unless `--max-report-age` is
set, so agents with a longer interval or heartbeat need a higher threshold.

Nothing triggers a reconcile when an agent falls silent, so after each fresh
report the controller requeues for the moment that report would go stale.

The condition messages carry no timestamps. A heartbeat that changes nothing
but `lastReportTime` therefore leaves the Target untouched. The time of the last
report is read from the report itself, for example by the UI's Agent card.

Staleness depends on the controller's clock only. The agent stamps
`lastReportTime` itself, so the controller uses it only to tell that the agent
wrote again: it remembers, in memory, when it first saw each value, and a
report goes stale once that is older than the threshold. The SolAr apiserver
keeps no `managedFields`, so there is no server-side write time to use instead.

After a controller restart or leader change, every report counts as written at
the moment the controller first sees it, except on Targets that were already
`Stale`: those stay `Stale` until the agent writes again. A silent agent whose
Target still showed `Reporting` is therefore marked `Stale` up to one threshold
later than without the restart.

## Binding status

The controller writes each listed binding's `Deployed` condition, across
namespaces, whether or not the Target has a report. These conditions come from
`status.resolvedBindings` alone:

| `Deployed` | Reason | When |
| ---------- | ------ | ---- |
| `False` | `Filtered` | The release resolver dropped the binding. Doesn't depend on the agent. |
| `Unknown` | `Unresolved` | The binding names the Target, but its Release, ComponentVersion or ReferenceGrant is missing. |

An accepted binding gets no `Deployed` condition yet; a `Filtered` or
`Unresolved` it had before, for example while its Release didn't exist yet, is
removed. The message comes from the list entry. A listed binding that no longer
exists, or whose `targetRef` now names another Target, is skipped: the list
survives the Target controller's early returns, so it can be out of date.

A binding that names the Target but isn't listed, for example because its
ReferenceGrant was revoked, must not keep an old `Deployed=True`. The controller
lists bindings from its cache, and sets any such binding that has a `Deployed`
condition to `Unknown`/`Unresolved`.

Binding status is written through the `/status` subresource and only when it
changed. `TargetReconciler` ignores binding updates that change neither the spec
nor the deletion timestamp, so these writes don't re-render the Target.

## Concurrency

`Target.status` has other writers (the Target and AgentCredential controllers),
all using a full status update. The controller reads the Target through an
uncached reader, so a concurrent write surfaces as a conflict, which is
returned and retried with a fresh read, rather than being overwritten.

## Watch Triggers

- A `TargetReport` is created, updated or deleted.
- A `Target` is created, so a report that existed before its Target is synced.
- A `Target`'s `status.resolvedBindings` changes. Its other status writes, this
  controller's included, are ignored.
- A requeue when the last fresh report would go stale.
