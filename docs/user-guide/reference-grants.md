# ReferenceGrants

A `ReferenceGrant` authorizes cross-namespace references between SolAr resources. Without a grant, controllers refuse to follow any reference that crosses a namespace boundary and set a `NotGranted` status condition on the referencing resource.

**Key invariant:** A `ReferenceGrant` always lives in the namespace that **owns the referenced resource** — not in the namespace of the resource doing the referencing. This gives each team full control over who may access their namespace's resources.

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReferenceGrant
metadata:
  namespace: <namespace-that-owns-the-referenced-resource>
spec:
  from:                              # who may reference
  - group: solar.opendefense.cloud
    kind: <referencing-resource-kind>
    namespace: <referencing-namespace>
  to:                                # what they may reference
  - group: solar.opendefense.cloud
    kind: <referenced-resource-kind>
```

Controllers check grants on every reconcile. Removing a grant immediately revokes access and sets `NotGranted` on all affected resources.

## K8s Cluster User

As a K8s Cluster User you own the namespace where your `Target` resources live. You create `ReferenceGrant`s here to allow provider-side resources to drive deployments to your clusters.

### Allow a Profile to match your Target

The `Profile` controller (running in the provider namespace) discovers Targets via label selectors and creates `ReleaseBinding` resources that bind a Release to each matching Target. For a Profile in the provider namespace to match a Target in your namespace, you must create a grant that permits both the `Profile` (discovery) and the resulting `ReleaseBinding` (rendering).

**ReferenceGrant** (in your namespace):

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReferenceGrant
metadata:
  name: allow-provider-target-access
  namespace: k8s-cluster-user        # your namespace
spec:
  from:
  - group: solar.opendefense.cloud
    kind: Profile
    namespace: k8s-cluster-provider
  - group: solar.opendefense.cloud
    kind: ReleaseBinding
    namespace: k8s-cluster-provider
  to:
  - group: solar.opendefense.cloud
    kind: Target
```

Both `from` entries are required: the Profile controller uses the `Profile` entry to discover Targets; the Target controller uses the `ReleaseBinding` entry to collect the cross-namespace bindings that drive rendering.

> **Debugging tip:** Unlike the Registry and ComponentVersion patterns, the Target is the *recipient* of a cross-namespace `ReleaseBinding` rather than the resource actively requesting cross-namespace access. If the `ReleaseBinding` entry is absent from the grant (or the grant doesn't exist), the Target controller treats the binding as invisible — the Target shows `ReleasesRendered=False` with reason `NoReleaseBindings` rather than `NotGranted`. If rendering does not start after a provider creates a binding for your Target, check that a `ReferenceGrant` with `kind: ReleaseBinding` in its `from` list exists in your namespace.

### Allow a RegistryBinding to reference your Target

A `RegistryBinding` in the provider namespace declares which OCI registry your Target may use as a source for pull credentials. To allow this, add a `RegistryBinding` entry to the same grant above (or to a separate grant):

```yaml
  from:
  - group: solar.opendefense.cloud
    kind: RegistryBinding
    namespace: k8s-cluster-provider
  to:
  - group: solar.opendefense.cloud
    kind: Target
```

In practice, all three entries (`Profile`, `ReleaseBinding`, `RegistryBinding`) are usually combined into a single grant for the provider namespace.

## K8s Cluster Provider

As a K8s Cluster Provider you own the namespace where `Registry` resources live. You create `ReferenceGrant`s here to allow Targets in user namespaces to reference your registries.

### Allow a Target to reference your Registry

`Target` resources may declare a `renderRegistryRef` pointing to a `Registry` in the provider namespace. To permit this cross-namespace reference, create a grant in your namespace:

**ReferenceGrant** (in the provider namespace):

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReferenceGrant
metadata:
  name: allow-target-registry-access
  namespace: k8s-cluster-provider    # your namespace
spec:
  from:
  - group: solar.opendefense.cloud
    kind: Target
    namespace: k8s-cluster-user
  to:
  - group: solar.opendefense.cloud
    kind: Registry
```

Without this grant, the Target controller sets `RegistryResolved=False` with reason `NotGranted`.

## App Catalog Maintainer

As an App Catalog Maintainer you own the namespace where `ComponentVersion` resources live. You create `ReferenceGrant`s here to allow `Release` resources in other namespaces to reference your component versions.

### Allow a Release to reference your ComponentVersion

**ReferenceGrant** (in the app-catalog namespace):

```yaml
apiVersion: solar.opendefense.cloud/v1alpha1
kind: ReferenceGrant
metadata:
  name: allow-release-cv-access
  namespace: app-catalog-maintainer  # your namespace
spec:
  from:
  - group: solar.opendefense.cloud
    kind: Release
    namespace: k8s-cluster-provider
  - group: solar.opendefense.cloud
    kind: Release
    namespace: k8s-cluster-user
  to:
  - group: solar.opendefense.cloud
    kind: ComponentVersion
```

**Release** with cross-namespace reference:

```yaml
spec:
  componentVersionRef:
    name: my-app-v1.2.3
  componentVersionNamespace: app-catalog-maintainer
```

Without this grant, the Release controller sets `ComponentVersionResolved=False` with reason `NotGranted`.

## Multi-tenant provisioning

In a multi-tenant, multi-cluster setup a new tenant namespace requires its own set of `ReferenceGrant`s — one in the cluster-user namespace (to allow Profile/ReleaseBinding/RegistryBinding access to the new Targets) and, for the App Catalog Maintainer pattern, an additional `from` entry or a new grant in the app-catalog namespace.

Creating these manually for every new namespace does not scale. The recommended approach is to manage `ReferenceGrant`s as part of your namespace provisioning pipeline — for example via a [Kyverno](https://kyverno.io) `generate` policy that automatically creates the required grants whenever a new namespace matching your tenant label is created, or by including them in a GitOps namespace template.

## Further reading

- [Roles](../developer-guide/roles.md) — full namespace model and permission matrix for each role
- [ADR-012](../developer-guide/adrs/012-ReferenceGrants.md) — decision record explaining why ReferenceGrants were chosen over cluster-scoped resource types
