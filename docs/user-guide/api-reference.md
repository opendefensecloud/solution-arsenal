# API Reference

## Packages
- [solar.opendefense.cloud/v1alpha1](#solaropendefensecloudv1alpha1)


## solar.opendefense.cloud/v1alpha1

Package v1alpha1 is the v1alpha1 version of the API.



#### BootstrapConfig



BootstrapConfig defines the render config for a bootstrap.



_Appears in:_
- [RenderTaskSpec](#rendertaskspec)
- [RendererConfig](#rendererconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `chart` _[ChartConfig](#chartconfig)_ | Chart is the ChartConfig for the rendered chart. |  |  |
| `input` _[BootstrapInput](#bootstrapinput)_ | Input is the input of the bootstrap. |  |  |


#### BootstrapInput



BootstrapInput defines the inputs to render a bootstrap.



_Appears in:_
- [BootstrapConfig](#bootstrapconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `releases` _object (keys:string, values:[ResolvedResourceAccess](#resolvedresourceaccess))_ |  |  |  |
| `userdata` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Userdata is additional data to be rendered into the bootstrap chart values. |  |  |


#### ChartConfig



ChartConfig defines parameters for the rendered chart.



_Appears in:_
- [BootstrapConfig](#bootstrapconfig)
- [ReleaseConfig](#releaseconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the chart. |  |  |
| `description` _string_ | Description is the description of the chart. |  |  |
| `version` _string_ | Version is the version of the chart. |  |  |
| `appVersion` _string_ | AppVersion is the version of the app. |  |  |


#### Component



Component represents an OCM component available in the solution catalog.



_Appears in:_
- [ComponentList](#componentlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ComponentSpec](#componentspec)_ |  |  |  |
| `status` _[ComponentStatus](#componentstatus)_ |  |  |  |


#### ComponentList



ComponentList contains a list of Component resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Component](#component) array_ |  |  |  |


#### ComponentSpec



ComponentSpec defines the desired state of a Component.
It contains metadata about an OCM component's repository location



_Appears in:_
- [Component](#component)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `scheme` _string_ | Scheme is the scheme to access the component. |  |  |
| `registry` _string_ | Registry is the registry where the component is stored. |  |  |
| `repository` _string_ | Repository is the repository where the component is stored. |  |  |
| `name` _string_ | Name is the raw OCM component name (e.g. "opendefense.cloud/arc").<br />Together with Scheme, Registry, Repository and a ComponentVersion's<br />Tag it forms the OCM component version reference the renderer resolves. |  |  |


#### ComponentStatus



ComponentStatus defines the observed state of a Component.



_Appears in:_
- [Component](#component)



#### ComponentVersion



ComponentVersion represents an OCM component available in the solution catalog.



_Appears in:_
- [ComponentVersionList](#componentversionlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ComponentVersionSpec](#componentversionspec)_ |  |  |  |
| `status` _[ComponentVersionStatus](#componentversionstatus)_ |  |  |  |


#### ComponentVersionList



ComponentVersionList contains a list of ComponentVersion resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ComponentVersion](#componentversion) array_ |  |  |  |


#### ComponentVersionSpec



ComponentVersionSpec defines the desired state of a ComponentVersion.



_Appears in:_
- [ComponentVersion](#componentversion)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `componentRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | ComponentRef is a reference to the parent Component. |  |  |
| `tag` _string_ | Tag is a version of the component. |  |  |
| `resources` _object (keys:string, values:[ResourceAccess](#resourceaccess))_ | Resources are Resources that are within the ComponentVersion. |  |  |
| `entrypoint` _[Entrypoint](#entrypoint)_ | Entrypoint is the entrypoint for deploying a ComponentVersion. |  |  |


#### ComponentVersionStatus



ComponentVersionStatus defines the observed state of a ComponentVersion.



_Appears in:_
- [ComponentVersion](#componentversion)



#### Entrypoint



Entrypoint defines the entrypoint for deploying a ComponentVersion.



_Appears in:_
- [ComponentVersionSpec](#componentversionspec)
- [ReleaseInput](#releaseinput)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resourceName` _string_ | ResourceName is the Name of the Resource to use as the entrypoint. |  |  |
| `type` _[EntrypointType](#entrypointtype)_ | Type of entrypoint. |  |  |


#### EntrypointType

_Underlying type:_ _string_

EntrypointType is the Type of Entrypoint.



_Appears in:_
- [Entrypoint](#entrypoint)

| Field | Description |
| --- | --- |
| `kro` |  |
| `helm` |  |


#### HelmResourceMetadata



HelmResourceMetadata contains metadata extracted from a Helm chart resource during discovery.



_Appears in:_
- [ResolvedResourceAccess](#resolvedresourceaccess)
- [ResourceAccess](#resourceaccess)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Helm chart. |  |  |
| `description` _string_ | Description of the Helm chart. |  |  |
| `version` _string_ | Version of the Helm chart. |  |  |
| `appVersion` _string_ | AppVersion of the application deployed by the chart. |  |  |


#### ObjectReference



ObjectReference references another resource by name, optionally in a different
namespace. When Namespace is empty, the referenced resource is assumed to live in
the same namespace as the referencing object. Cross-namespace references require a
ReferenceGrant in the referenced resource's namespace that grants access to the
referencing object's namespace.



_Appears in:_
- [RegistryBindingSpec](#registrybindingspec)
- [ReleaseBindingSpec](#releasebindingspec)
- [ReleaseSpec](#releasespec)
- [RenderArtifactSpec](#renderartifactspec)
- [RenderBindingSpec](#renderbindingspec)
- [TargetSpec](#targetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the referenced resource. |  | Required: \{\} <br /> |
| `namespace` _string_ | Namespace is the namespace of the referenced resource. If empty, the resource is<br />assumed to be in the same namespace as the referencing object. |  | Optional: \{\} <br /> |


#### Profile



Profile represents the link between a Release and a set of matching Targets the Release is
intended to be deployed to.

Deletion is a destructive, cascading operation: deleting a Profile deletes all owned
ReleaseBindings. To remove a Profile without triggering undeployment, first remove or relabel
all matching Targets so the Profile controller deletes the ReleaseBindings itself, then delete
the Profile once it has no owned bindings.



_Appears in:_
- [ProfileList](#profilelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ProfileSpec](#profilespec)_ |  |  |  |
| `status` _[ProfileStatus](#profilestatus)_ |  |  |  |


#### ProfileList



ProfileList contains a list of Profile resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Profile](#profile) array_ |  |  |  |


#### ProfileSpec



ProfileSpec defines the desired state of a Profile.
It points to a Release and defines target selection criteria for
Targets this Release is intended to be deployed to.



_Appears in:_
- [Profile](#profile)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `releaseRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | ReleaseRef is a reference to a Release.<br />It points to the Release that is intended to be deployed to all Targets identified<br />by the TargetSelector. |  | Required: \{\} <br /> |
| `targetSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#labelselector-v1-meta)_ | TargetSelector is a label-based filter to identify the Targets this Release is<br />intended to be deployed to. |  | Optional: \{\} <br /> |
| `userdata` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Userdata contains arbitrary custom data or configuration which is passed to all<br />Targets associated with this Profile. |  | Optional: \{\} <br /> |


#### ProfileStatus



ProfileStatus defines the observed state of a Profile.



_Appears in:_
- [Profile](#profile)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `matchedTargets` _integer_ | MatchedTargets is the total number of Targets matching the target selection criteria. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of the Profile's state. |  |  |




#### ReferenceGrant



ReferenceGrant grants namespaces listed in From permission to reference resource types
listed in To within the namespace where this ReferenceGrant lives.

This enables cross-namespace use-cases such as a Profile in one namespace matching
Targets in another namespace, or a ReleaseBinding referencing a Registry defined
in a shared infrastructure namespace.



_Appears in:_
- [ReferenceGrantList](#referencegrantlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ReferenceGrantSpec](#referencegrantspec)_ |  |  |  |


#### ReferenceGrantFromSubject



ReferenceGrantFromSubject identifies the group, kind, and namespace of a resource that
is permitted to reference resources in the namespace where the ReferenceGrant lives.



_Appears in:_
- [ReferenceGrantSpec](#referencegrantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `group` _string_ | Group is the API group of the referencing resource.<br />Use "" for the core API group. |  | Required: \{\} <br /> |
| `kind` _string_ | Kind is the kind of the referencing resource (e.g. "Profile", "Target"). |  | Required: \{\} <br /> |
| `namespace` _string_ | Namespace is the namespace of the referencing resource.<br />A single namespace is allowed per From entry to avoid overly broad grants. |  | Required: \{\} <br /> |


#### ReferenceGrantList



ReferenceGrantList contains a list of ReferenceGrant resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ReferenceGrant](#referencegrant) array_ |  |  |  |


#### ReferenceGrantSpec



ReferenceGrantSpec defines the desired state of a ReferenceGrant.



_Appears in:_
- [ReferenceGrant](#referencegrant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `from` _[ReferenceGrantFromSubject](#referencegrantfromsubject) array_ | From is the list of resources that are permitted to reference resources in this namespace.<br />Each entry specifies the group, kind, and namespace of an allowed referencing resource. |  | MinItems: 1 <br /> |
| `to` _[ReferenceGrantToTarget](#referencegranttotarget) array_ | To is the list of resource types in this namespace that may be referenced from the<br />resources listed in From. |  | MinItems: 1 <br /> |


#### ReferenceGrantToTarget



ReferenceGrantToTarget specifies the group and kind of resource that may be referenced.
Resource names are intentionally excluded: a namespace-scoped grant already limits
the blast radius, and name restrictions rarely provide meaningful security.



_Appears in:_
- [ReferenceGrantSpec](#referencegrantspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `group` _string_ | Group is the API group of the referenced resource.<br />Use "" for the core API group. |  | Required: \{\} <br /> |
| `kind` _string_ | Kind is the kind of the referenced resource (e.g. "Target", "Registry"). |  | Required: \{\} <br /> |


#### Registry



Registry represents an OCI registry that can be used as a source or destination for artifacts.



_Appears in:_
- [RegistryList](#registrylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RegistrySpec](#registryspec)_ |  |  |  |
| `status` _[RegistryStatus](#registrystatus)_ |  |  |  |


#### RegistryBinding



RegistryBinding declares that a specific Target is allowed to use a specific Registry.



_Appears in:_
- [RegistryBindingList](#registrybindinglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RegistryBindingSpec](#registrybindingspec)_ |  |  |  |
| `status` _[RegistryBindingStatus](#registrybindingstatus)_ |  |  |  |


#### RegistryBindingList



RegistryBindingList contains a list of RegistryBinding resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[RegistryBinding](#registrybinding) array_ |  |  |  |


#### RegistryBindingSpec



RegistryBindingSpec defines the desired state of a RegistryBinding.



_Appears in:_
- [RegistryBinding](#registrybinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `targetRef` _[ObjectReference](#objectreference)_ | TargetRef references the Target this binding applies to. When Namespace is set, the<br />Target resides in a different namespace than this RegistryBinding; cross-namespace<br />references require a ReferenceGrant in the Target's namespace that permits this<br />RegistryBinding's namespace. |  |  |
| `registryRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | RegistryRef references the Registry being bound. |  |  |


#### RegistryBindingStatus



RegistryBindingStatus defines the observed state of a RegistryBinding.



_Appears in:_
- [RegistryBinding](#registrybinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a RegistryBinding's state. |  | Optional: \{\} <br /> |


#### RegistryList



RegistryList contains a list of Registry resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Registry](#registry) array_ |  |  |  |


#### RegistrySpec



RegistrySpec defines the desired state of a Registry.



_Appears in:_
- [Registry](#registry)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `hostname` _string_ | Hostname is the registry endpoint (e.g. "registry.example.com:5000"). |  |  |
| `plainHTTP` _boolean_ | PlainHTTP uses HTTP instead of HTTPS for connections to this registry. |  | Optional: \{\} <br /> |
| `solarSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | SolarSecretRef references a Secret in the same namespace with credentials<br />to access this registry from the SolAr cluster. Required if this registry<br />is used as a render target. |  | Optional: \{\} <br /> |
| `targetPullSecretName` _string_ | TargetPullSecretName is the name of the Secret on the target cluster that<br />contains credentials to pull from this registry. SolAr renders this name<br />into target manifests (e.g. Flux OCIRepository.spec.secretRef.name) but<br />never reads the Secret itself. The cluster maintainer must provision a<br />Secret with this name on each target. Omit for anonymous pull. |  | Optional: \{\} <br /> |
| `flavor` _string_ | Flavor identifies the registry type for discovery webhook routing (e.g. "zot").<br />Required when WebhookPath is set. |  | Optional: \{\} <br /> |
| `webhookPath` _string_ | WebhookPath is the HTTP path on which the discovery worker listens for<br />push notifications from this registry. Leave empty to disable webhook-based<br />discovery; set ScanInterval to enable scan mode instead. |  | Optional: \{\} <br /> |
| `scanInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#duration-v1-meta)_ | ScanInterval controls how often the discovery worker performs a full scan<br />of this registry. Leave unset to disable scan mode entirely. |  | Optional: \{\} <br /> |


#### RegistryStatus



RegistryStatus defines the observed state of a Registry.



_Appears in:_
- [Registry](#registry)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a Registry's state. |  | Optional: \{\} <br /> |


#### Release



Release represents a specific deployment instance of a component.
It combines a component version with deployment values and configuration for a particular use case.



_Appears in:_
- [ReleaseList](#releaselist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ReleaseSpec](#releasespec)_ |  |  |  |
| `status` _[ReleaseStatus](#releasestatus)_ |  |  |  |


#### ReleaseBinding



ReleaseBinding declares that a Release should be deployed to a Target.



_Appears in:_
- [ReleaseBindingList](#releasebindinglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ReleaseBindingSpec](#releasebindingspec)_ |  |  |  |
| `status` _[ReleaseBindingStatus](#releasebindingstatus)_ |  |  |  |


#### ReleaseBindingList



ReleaseBindingList contains a list of ReleaseBinding resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ReleaseBinding](#releasebinding) array_ |  |  |  |


#### ReleaseBindingSpec



ReleaseBindingSpec defines the desired state of a ReleaseBinding.



_Appears in:_
- [ReleaseBinding](#releasebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `targetRef` _[ObjectReference](#objectreference)_ | TargetRef references the Target this release is bound to. When Namespace is set, the<br />Target resides in a different namespace than this ReleaseBinding; cross-namespace<br />references require a ReferenceGrant in the target's namespace that grants access to<br />this ReleaseBinding's namespace. |  |  |
| `releaseRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | ReleaseRef references the Release to deploy. |  |  |


#### ReleaseBindingStatus



ReleaseBindingStatus defines the observed state of a ReleaseBinding.



_Appears in:_
- [ReleaseBinding](#releasebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a ReleaseBinding's state. |  | Optional: \{\} <br /> |


#### ReleaseComponent



ReleaseComponent is a reference to a component.



_Appears in:_
- [ReleaseInput](#releaseinput)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the component. |  |  |
| `ref` _string_ | Ref is the OCM component version reference the renderer resolves to fetch<br />the component's helm values template, in the form<br />"[<protocol>://]<host>/<namespace>//<component-name>:<version>".<br />Empty disables values-template rendering for this release. |  | Optional: \{\} <br /> |


#### ReleaseConfig



ReleaseConfig defines the render config for a release.



_Appears in:_
- [RenderTaskSpec](#rendertaskspec)
- [RendererConfig](#rendererconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `chart` _[ChartConfig](#chartconfig)_ | Chart is the ChartConfig for the rendered chart. |  |  |
| `input` _[ReleaseInput](#releaseinput)_ | Input is the input of the release. |  |  |
| `targetNamespace` _string_ | TargetNamespace is the namespace the Component gets deployed to. |  |  |
| `values` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Values are additional values to be rendered into the release chart. |  |  |


#### ReleaseInput



ReleaseInput defines the inputs to render a release.



_Appears in:_
- [ReleaseConfig](#releaseconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `component` _[ReleaseComponent](#releasecomponent)_ | Component is a reference to the component. |  |  |
| `resources` _object (keys:string, values:[ResolvedResourceAccess](#resolvedresourceaccess))_ | Resources is the map of resolved resources in the component. |  |  |
| `entrypoint` _[Entrypoint](#entrypoint)_ | Entrypoint is the resource to be used as an entrypoint for deployment. |  |  |
| `pullSecrets` _object (keys:string, values:string)_ | PullSecrets maps a registry hostname to the name of the pull secret on the<br />target cluster, resolved from the target's RegistryBindings. |  | Optional: \{\} <br /> |


#### ReleaseList



ReleaseList contains a list of Release resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Release](#release) array_ |  |  |  |


#### ReleaseSpec



ReleaseSpec defines the desired state of a Release.
It specifies which component version to release and its deployment configuration.



_Appears in:_
- [Release](#release)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `componentVersionRef` _[ObjectReference](#objectreference)_ | ComponentVersionRef is a reference to the ComponentVersion to be released. It points to<br />the specific version of a component that this release is based on. When Namespace is set,<br />the ComponentVersion resides in another namespace; cross-namespace references require a<br />ReferenceGrant in the ComponentVersion's namespace that grants access to this Release's<br />namespace. |  |  |
| `targetNamespace` _string_ | TargetNamespace is the namespace the ComponentVersion gets deployed to. |  | Optional: \{\} <br /> |
| `uniqueName` _string_ | UniqueName is a logical identifier that ensures only one Release of this<br />component is deployed per Target when multiple Profiles match.<br />If not set, it defaults to the parent Component name (derived from the<br />referenced ComponentVersion). Immutable once set. |  | Optional: \{\} <br /> |
| `antiAffinity` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#labelselector-v1-meta)_ | AntiAffinity defines exclusion rules. If another Release matching this<br />label selector is already bound to the same Target, this Release should<br />not be deployed there (or a conflict condition should be raised). |  | Optional: \{\} <br /> |
| `values` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Values contains deployment-specific values or configuration for the release.<br />These values override defaults from the component version and are used during deployment. |  | Optional: \{\} <br /> |
| `failedJobTTL` _integer_ | failedJobTTL is the TTL in seconds after which a failed render job and its secrets are cleaned up.<br />After this duration, the Kubernetes TTL controller will delete the Job and the controller will delete<br />the Secrets (ConfigSecret, AuthSecret). On success, Job and Secrets are deleted immediately.<br />If not set, defaults to 3600 (1 hour). |  | Optional: \{\} <br /> |
| `priority` _integer_ | Priority determines which Release takes precedence when multiple Releases<br />share the same unique name on a Target. Higher values indicate higher priority.<br />If not set, defaults to 0. |  | Optional: \{\} <br /> |


#### ReleaseStatus



ReleaseStatus defines the observed state of a Release.



_Appears in:_
- [Release](#release)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a Release's state. |  | Optional: \{\} <br /> |
| `renderTaskRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectreference-v1-core)_ | RenderTaskRef is a reference to the RenderTask responsible for this Release. |  | Optional: \{\} <br /> |
| `effectiveUniqueName` _string_ | EffectiveUniqueName is the unique name used for deduplication on Targets.<br />Equals Spec.UniqueName when set; otherwise the parent Component name derived<br />from the referenced ComponentVersion. |  | Optional: \{\} <br /> |


#### RenderArtifact



RenderArtifact represents a successfully pushed OCI artifact produced by a RenderTask.



_Appears in:_
- [RenderArtifactList](#renderartifactlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RenderArtifactSpec](#renderartifactspec)_ |  |  |  |
| `status` _[RenderArtifactStatus](#renderartifactstatus)_ |  |  |  |


#### RenderArtifactList



RenderArtifactList contains a list of RenderArtifact resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[RenderArtifact](#renderartifact) array_ |  |  |  |


#### RenderArtifactSpec



RenderArtifactSpec holds the OCI coordinates of a successfully pushed artifact.



_Appears in:_
- [RenderArtifact](#renderartifact)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `baseURL` _string_ | BaseURL is the registry's base URL (e.g. "registry.example.com:5000"). |  | MinLength: 1 <br /> |
| `repository` _string_ | Repository is the repository path within the registry. |  | MinLength: 1 <br /> |
| `tag` _string_ | Tag is the OCI tag that was pushed. |  | MinLength: 1 <br /> |
| `renderTaskRef` _string_ | RenderTaskRef is the name of the RenderTask that produced this artifact. |  |  |
| `registryRef` _[ObjectReference](#objectreference)_ | RegistryRef references the Registry that owns the credentials used to push (and<br />later delete) this artifact's OCI tag. When Namespace is empty, the Registry is<br />resolved in the RenderArtifact's own namespace; a non-empty Namespace identifies a<br />different namespace and requires a ReferenceGrant there permitting access, mirroring<br />how Target resolves its RenderRegistryRef. That grant must name this kind: from[].kind<br />"RenderArtifact" with the RenderArtifact's namespace and to[].kind "Registry". The<br />Target's own grant is deliberately not accepted — the field is meant to be<br />controller-owned (copied from a RenderBinding the Target controller populated from<br />Target.Spec.RenderRegistryRef), but the API does not enforce that, so a hand-authored<br />artifact would otherwise borrow the Target's credentials.<br />RenderArtifact never stores Secret- or<br />PlainHTTP-identifying information directly: both are read live from the referenced<br />Registry whenever credentials are needed, so a Registry's credentials or transport<br />settings can change without ever going stale on the artifact. |  | Optional: \{\} <br /> |


#### RenderArtifactStatus



RenderArtifactStatus holds the observed state of a RenderArtifact.



_Appears in:_
- [RenderArtifact](#renderartifact)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `chartURL` _string_ | ChartURL is the fully-qualified OCI reference for this artifact. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a RenderArtifact's state. |  | Optional: \{\} <br /> |


#### RenderBinding



RenderBinding declares that a consumer resource (e.g. a Target) is using a RenderArtifact.



_Appears in:_
- [RenderBindingList](#renderbindinglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RenderBindingSpec](#renderbindingspec)_ |  |  |  |


#### RenderBindingList



RenderBindingList contains a list of RenderBinding resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[RenderBinding](#renderbinding) array_ |  |  |  |


#### RenderBindingSpec



RenderBindingSpec links a consumer resource to a RenderArtifact for ref-counting.



_Appears in:_
- [RenderBinding](#renderbinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `renderArtifactRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | RenderArtifactRef is the name of the RenderArtifact in the same namespace. |  |  |
| `ownerKind` _string_ | OwnerKind is the kind of the consuming resource (e.g. "Target"). |  | MinLength: 1 <br /> |
| `ownerName` _string_ | OwnerName is the name of the consuming resource. |  | MinLength: 1 <br /> |
| `ownerNamespace` _string_ | OwnerNamespace is the namespace of the consuming resource. |  | MinLength: 1 <br /> |
| `registryRef` _[ObjectReference](#objectreference)_ | RegistryRef references the Registry this binding's owner currently resolves for<br />pushing the shared RenderArtifact. The RenderArtifact controller re-pins<br />RenderArtifact.Spec.RegistryRef from a surviving RenderBinding's value whenever a<br />binding is removed, so the artifact always resolves credentials through a Registry<br />belonging to a consumer that still exists. RenderArtifact/RenderBinding never store<br />Secret-identifying information directly, only a reference to the Registry that owns<br />the credentials, resolved fresh at use time, mirroring how Target resolves its own<br />push credentials. |  | Optional: \{\} <br /> |




#### RenderTask



RenderTask manages a rendering job



_Appears in:_
- [RenderTaskList](#rendertasklist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RenderTaskSpec](#rendertaskspec)_ |  |  |  |
| `status` _[RenderTaskStatus](#rendertaskstatus)_ |  |  |  |


#### RenderTaskList



RenderTaskList contains a list of RenderTask resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[RenderTask](#rendertask) array_ |  |  |  |


#### RenderTaskSpec



RenderTaskSpec holds the specification for a RenderTask



_Appears in:_
- [RenderTask](#rendertask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[RendererConfigType](#rendererconfigtype)_ | Type defines the output type of the renderer. |  |  |
| `release` _[ReleaseConfig](#releaseconfig)_ | ReleaseConfig is a config for a release. |  |  |
| `bootstrap` _[BootstrapConfig](#bootstrapconfig)_ | BootstrapConfig is a config for a bootstrap. |  |  |
| `signing` _[SigningConfig](#signingconfig)_ | Signing configures cosign key-based signing of the pushed artifact.<br />Nil disables signing. |  | Optional: \{\} <br /> |
| `repository` _string_ | Repository is the Repository where the chart will be pushed to (e.g. charts/mychart) |  |  |
| `tag` _string_ | Tag is the Tag of the helm chart to be pushed.<br />Make sure that the tag matches the version in Chart.yaml, otherwise helm<br />will error before pushing. |  |  |
| `baseURL` _string_ | BaseURL is the registry URL to push the rendered chart to (e.g. "registry.example.com:5000"). |  |  |
| `pushSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | PushSecretRef references a Secret in the same namespace with registry credentials<br />for pushing the rendered chart. |  | Optional: \{\} <br /> |
| `sourceSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | SourceSecretRef references a Secret in the same namespace with registry<br />credentials for reading the OCM component the release is built from. The<br />source registry may differ from the push registry. |  | Optional: \{\} <br /> |
| `plainHTTP` _boolean_ | PlainHTTP uses HTTP instead of HTTPS for OCI registry connections. |  | Optional: \{\} <br /> |
| `failedJobTTL` _integer_ | failedJobTTL is the TTL in seconds after which a failed render job and its secrets are cleaned up.<br />After this duration, the Kubernetes TTL controller will delete the Job and the controller will delete<br />the Secrets (ConfigSecret, AuthSecret). On success, Job and Secrets are deleted immediately.<br />If not set, defaults to 3600 (1 hour). |  | Optional: \{\} <br /> |
| `ownerName` _string_ | OwnerName is the name of the resource that created this RenderTask. |  | MinLength: 1 <br /> |
| `ownerNamespace` _string_ | OwnerNamespace is the namespace of the resource that created this RenderTask. |  | MinLength: 1 <br /> |
| `ownerKind` _string_ | OwnerKind is the kind of the resource that created this RenderTask (e.g. Release, Target). |  | MinLength: 1 <br /> |


#### RenderTaskStatus



RenderTaskStatus holds the status of the rendering process



_Appears in:_
- [RenderTask](#rendertask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a RenderTask's state. |  | Optional: \{\} <br /> |
| `jobRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectreference-v1-core)_ | JobRef is a reference to the Job that is executing the rendering. |  | Optional: \{\} <br /> |
| `configSecretRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectreference-v1-core)_ | ConfigSecretRef is a reference to the Secret containing the renderer configuration. |  | Optional: \{\} <br /> |
| `chartURL` _string_ | ChartURL represents the URL of where the rendered chart was pushed to. |  | Optional: \{\} <br /> |


#### RendererConfig



RendererConfig defines the configuration for the renderer.



_Appears in:_
- [RenderTaskSpec](#rendertaskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[RendererConfigType](#rendererconfigtype)_ | Type defines the output type of the renderer. |  |  |
| `release` _[ReleaseConfig](#releaseconfig)_ | ReleaseConfig is a config for a release. |  |  |
| `bootstrap` _[BootstrapConfig](#bootstrapconfig)_ | BootstrapConfig is a config for a bootstrap. |  |  |
| `signing` _[SigningConfig](#signingconfig)_ | Signing configures cosign key-based signing of the pushed artifact.<br />Nil disables signing. |  | Optional: \{\} <br /> |


#### RendererConfigType

_Underlying type:_ _string_

RendererConfigType is the output type of the renderer.



_Appears in:_
- [RenderTaskSpec](#rendertaskspec)
- [RendererConfig](#rendererconfig)

| Field | Description |
| --- | --- |
| `bootstrap` |  |
| `release` |  |
| `profile` |  |


#### ResolvedResourceAccess



ResolvedResourceAccess extends ResourceAccess with pull secret information
resolved from RegistryBindings at render time.



_Appears in:_
- [BootstrapInput](#bootstrapinput)
- [ReleaseInput](#releaseinput)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ | Repository of the Resource. |  |  |
| `insecure` _boolean_ | Insecure switches TLS/HTTPS off if true |  |  |
| `tag` _string_ | Tag of the Resource. |  |  |
| `helm` _[HelmResourceMetadata](#helmresourcemetadata)_ | Helm contains metadata for Helm chart resources, populated during discovery. |  |  |
| `pullSecretName` _string_ | PullSecretName is the name of the pull secret on the target cluster for<br />this resource's registry. Resolved from Registry.spec.targetPullSecretName<br />via RegistryBinding. Empty means anonymous pull. |  |  |


#### ResourceAccess



ResourceAccess defines how a Resource can be accessed along with optional metadata.



_Appears in:_
- [ComponentVersionSpec](#componentversionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ | Repository of the Resource. |  |  |
| `insecure` _boolean_ | Insecure switches TLS/HTTPS off if true |  |  |
| `tag` _string_ | Tag of the Resource. |  |  |
| `helm` _[HelmResourceMetadata](#helmresourcemetadata)_ | Helm contains metadata for Helm chart resources, populated during discovery. |  |  |


#### SigningConfig



SigningConfig configures cosign key-based signing of the rendered artifact.
The key password is read from the COSIGN_PASSWORD environment variable.



_Appears in:_
- [RenderTaskSpec](#rendertaskspec)
- [RendererConfig](#rendererconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `keyPath` _string_ | KeyPath is the path to the cosign private key mounted into the render job. |  |  |


#### Target



Target represents a deployment target environment.
It defines the intended state of releases and configuration for a specific deployment target,
such as a cluster or environment.



_Appears in:_
- [TargetList](#targetlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[TargetSpec](#targetspec)_ |  |  |  |
| `status` _[TargetStatus](#targetstatus)_ |  |  |  |


#### TargetList



TargetList contains a list of Target resources.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  | Optional: \{\} <br /> |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  | Optional: \{\} <br /> |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Target](#target) array_ |  |  |  |


#### TargetSpec



TargetSpec defines the desired state of a Target.
It specifies the render registry and configuration for this deployment target.



_Appears in:_
- [Target](#target)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `renderRegistryRef` _[ObjectReference](#objectreference)_ | RenderRegistryRef references the Registry to push rendered desired state to.<br />The referenced Registry must have SolarSecretRef set for rendering to succeed.<br />When Namespace is set, the Registry resides in a different namespace than this<br />Target; cross-namespace references require a ReferenceGrant in the Registry's<br />namespace that grants access to this Target's namespace. |  |  |
| `userdata` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Userdata contains arbitrary custom data or configuration specific to this target.<br />This enables target-specific customization and deployment parameters. |  | Optional: \{\} <br /> |


#### TargetStatus



TargetStatus defines the observed state of a Target.



_Appears in:_
- [Target](#target)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bootstrapVersion` _integer_ | BootstrapVersion is a monotonically increasing counter used as the bootstrap<br />chart version. It is incremented each time the bootstrap chart is re-rendered,<br />e.g. when the set of bound releases changes. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a Target's state. |  | Optional: \{\} <br /> |


