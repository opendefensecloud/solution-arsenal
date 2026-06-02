---
status: accepted
date: 2026-03-06
---

# Unique Release Name

## Context and Problem Statement

As a CPaaS provider we need to manage addons across a wide range of Kubernetes Clusters.

Based on external factors we want to match profiles, e.g.:
- There is a Release for Kyverno V1 and it supports K8s >1.25.
- There is another Release for Kyverno V2 and it supports K8s >1.33.

We now have two profiles matching our Clusters with K8s version >1.33 and others.

Kyverno should only be rolled out exactly once with higher versions taking precedence and allow in-place upgrades.

Without conflict resolution, both profiles match the same Target and both Kyverno releases are deployed — causing a duplicate installation:

```mermaid
flowchart LR
    comp[Component Kyverno]
    cv1[ComponentVersion 1.0.0]
    cv2[ComponentVersion 2.0.0]
    rel1[Release 1]
    rel2[Release 2]
    p1["Profile 1\nk8s > 1.25"]
    p2["Profile 2\nk8s > 1.33"]
    target["Target\nk8s 1.35"]

    subgraph cluster["k8s Cluster 💥"]
        direction TB
        k1[Kyverno 1.0.0]
        k2[Kyverno 2.0.0]
    end

    comp --> cv1 --> rel1 --> p1
    comp --> cv2 --> rel2 --> p2
    p1 -->|matches| target
    p2 -->|matches| target
    target -->|renders both| k1
    target -->|renders both| k2
```

## Decision Outcome

- `Release.Spec.UniqueName` is a logical identifier that groups releases representing the same component. When multiple releases share a `uniqueName`, only one is deployed per Target.
    - If not set, the Target controller uses the parent Component name (from the referenced ComponentVersion) as the effective deduplication key at reconcile time. The field itself remains empty in storage.
    - Immutable once set.
- Releases carry a `Priority` field. When multiple releases share the same `uniqueName` on a Target, the one with the highest priority wins.
    - If priorities are equal, the conflict is broken deterministically by the namespace-qualified ReleaseBinding name in ascending alphabetical order.
- Releases can define `AntiAffinity` rules (label selectors). If another Release already accepted for a Target matches the selector — or vice versa — the lower-priority Release is excluded.
- The resolver runs in the Target controller after all ReleaseBindings are collected and before any RenderTask is created.
- The `uniqueName` doubles as the identity of a release within a Target's bootstrap chart: it is the map key under which the release is registered in the bootstrap input (and thus the suffix of the generated FluxCD resource names). The Kubernetes Release object name is **not** suitable here because it is not unique across namespaces — with cross-namespace ReleaseBindings, two namespaces can each define a `my-release`, and keying on the object name would let one silently overwrite the other in the bootstrap. Keying on `uniqueName` is safe because the resolver guarantees it is unique among accepted releases.

## Solution

### Deduplication by priority

Two releases share `uniqueName: kyverno`. The Target controller keeps only the higher-priority one:

```mermaid
flowchart LR
    comp[Component Kyverno]
    cv1[ComponentVersion 1.0.0]
    cv2[ComponentVersion 2.0.0]
    rel1["Release 1\nuniqueName: kyverno\npriority: 1"]
    rel2["Release 2\nuniqueName: kyverno\npriority: 2"]
    p1["Profile 1\nk8s > 1.25"]
    p2["Profile 2\nk8s > 1.33"]
    target["Target\nk8s 1.35"]
    resolver{{"Release Resolver\nkyverno → Release 2 wins"}}
    rt[RenderTask]

    subgraph cluster["k8s Cluster ✓"]
        k[Kyverno 2.0.0]
    end

    comp --> cv1 --> rel1 --> p1
    comp --> cv2 --> rel2 --> p2
    p1 -->|matches| target
    p2 -->|matches| target
    target --> resolver
    resolver -->|accepted| rt --> k
    rel1 -.->|filtered: lower priority| resolver
```

### Anti-affinity

A Release can declare that it must not be co-deployed with releases matching a given label selector. The resolver enforces this bidirectionally:

```mermaid
flowchart LR
    istio["Release Istio\nlabels: category=service-mesh\npriority: 10"]
    linkerd["Release Linkerd\nantiAffinity: category=service-mesh\npriority: 5"]
    target[Target]
    resolver{{"Release Resolver"}}
    rt[RenderTask]

    subgraph cluster["k8s Cluster ✓"]
        i[Istio]
    end

    istio -->|bound via Profile| target
    linkerd -->|bound via Profile| target
    target --> resolver
    resolver -->|accepted: higher priority| rt --> i
    linkerd -.->|filtered: anti-affinity conflict with Istio| resolver
```
