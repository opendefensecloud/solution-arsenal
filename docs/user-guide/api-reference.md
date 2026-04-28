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
| `releases` _object (keys:string, values:[ResourceAccess](#resourceaccess))_ |  |  |  |
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
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ComponentSpec](#componentspec)_ |  |  |  |
| `status` _[ComponentStatus](#componentstatus)_ |  |  |  |




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
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ComponentVersionSpec](#componentversionspec)_ |  |  |  |
| `status` _[ComponentVersionStatus](#componentversionstatus)_ |  |  |  |




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


#### Profile



Profile represents the link between a Release and a set of matching Targets the Release is
intended to be deployed to.



_Appears in:_
- [ProfileList](#profilelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ProfileSpec](#profilespec)_ |  |  |  |
| `status` _[ProfileStatus](#profilestatus)_ |  |  |  |




#### ProfileSpec



ProfileSpec defines the desired state of a Profile.
It points to a Release and defines target selection criteria for
Targets this Release is intended to be deployed to.



_Appears in:_
- [Profile](#profile)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `releaseRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | ReleaseRef is a reference to a Release.<br />It points to the Release that is intended to be deployed to all Targets identified<br />by the TargetSelector. |  | Required: \{\} <br /> |
| `targetSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#labelselector-v1-meta)_ | TargetSelector is a label-based filter to identify the Targets this Release is<br />intended to be deployed to. |  |  |
| `userdata` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Userdata contains arbitrary custom data or configuration which is passed to all<br />Targets associated with this Profile. |  |  |


#### ProfileStatus



ProfileStatus defines the observed state of a Profile.



_Appears in:_
- [Profile](#profile)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `matchedTargets` _integer_ | MatchedTargets is the total number of Targets matching the target selection criteria. |  |  |
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
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
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
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RegistrySpec](#registryspec)_ |  |  |  |
| `status` _[RegistryStatus](#registrystatus)_ |  |  |  |


#### RegistryBinding



RegistryBinding declares that a specific Target is allowed to use a specific Registry.



_Appears in:_
- [RegistryBindingList](#registrybindinglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RegistryBindingSpec](#registrybindingspec)_ |  |  |  |
| `status` _[RegistryBindingStatus](#registrybindingstatus)_ |  |  |  |




#### RegistryBindingSpec



RegistryBindingSpec defines the desired state of a RegistryBinding.



_Appears in:_
- [RegistryBinding](#registrybinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `targetRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | TargetRef references the Target this binding applies to. |  |  |
| `registryRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | RegistryRef references the Registry being bound. |  |  |


#### RegistryBindingStatus



RegistryBindingStatus defines the observed state of a RegistryBinding.



_Appears in:_
- [RegistryBinding](#registrybinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a RegistryBinding's state. |  |  |




#### RegistrySpec



RegistrySpec defines the desired state of a Registry.



_Appears in:_
- [Registry](#registry)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `hostname` _string_ | Hostname is the registry endpoint (e.g. "registry.example.com:5000"). |  |  |
| `plainHTTP` _boolean_ | PlainHTTP uses HTTP instead of HTTPS for connections to this registry. |  |  |
| `solarSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | SolarSecretRef references a Secret in the same namespace with credentials<br />to access this registry from the SolAr cluster. Required if this registry<br />is used as a render target. |  |  |
| `targetSecretRef` _[TargetSecretReference](#targetsecretreference)_ | TargetSecretRef describes where the credentials secret lives in the target cluster.<br />Used by the target agent for pull access. |  |  |


#### RegistryStatus



RegistryStatus defines the observed state of a Registry.



_Appears in:_
- [Registry](#registry)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a Registry's state. |  |  |


#### Release



Release represents a specific deployment instance of a component.
It combines a component version with deployment values and configuration for a particular use case.



_Appears in:_
- [ReleaseList](#releaselist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ReleaseSpec](#releasespec)_ |  |  |  |
| `status` _[ReleaseStatus](#releasestatus)_ |  |  |  |


#### ReleaseBinding



ReleaseBinding declares that a Release should be deployed to a Target.



_Appears in:_
- [ReleaseBindingList](#releasebindinglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ReleaseBindingSpec](#releasebindingspec)_ |  |  |  |
| `status` _[ReleaseBindingStatus](#releasebindingstatus)_ |  |  |  |




#### ReleaseBindingSpec



ReleaseBindingSpec defines the desired state of a ReleaseBinding.



_Appears in:_
- [ReleaseBinding](#releasebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `targetRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | TargetRef references the Target this release is bound to. |  |  |
| `targetNamespace` _string_ | TargetNamespace is the namespace of the Target when it resides in a different namespace<br />than this ReleaseBinding. If empty, the Target is assumed to be in the same namespace.<br />Cross-namespace references require a ReferenceGrant in the target's namespace that grants<br />access to this ReleaseBinding's namespace. |  |  |
| `releaseRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | ReleaseRef references the Release to deploy. |  |  |


#### ReleaseBindingStatus



ReleaseBindingStatus defines the observed state of a ReleaseBinding.



_Appears in:_
- [ReleaseBinding](#releasebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a ReleaseBinding's state. |  |  |


#### ReleaseComponent



ReleaseComponent is a reference to a component.



_Appears in:_
- [ReleaseInput](#releaseinput)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the component. |  |  |


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
| `resources` _object (keys:string, values:[ResourceAccess](#resourceaccess))_ | Resources is the map of resources in the component. |  |  |
| `entrypoint` _[Entrypoint](#entrypoint)_ | Entrypoint is the resource to be used as an entrypoint for deployment. |  |  |




#### ReleaseSpec



ReleaseSpec defines the desired state of a Release.
It specifies which component version to release and its deployment configuration.



_Appears in:_
- [Release](#release)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `componentVersionRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | ComponentVersionRef is a reference to the ComponentVersion to be released.<br />It points to the specific version of a component that this release is based on. |  |  |
| `componentVersionNamespace` _string_ | ComponentVersionNamespace is the namespace where ComponentVersionRef is resolved.<br />When set, the Release references a ComponentVersion in another namespace.<br />Cross-namespace references require a ReferenceGrant in the ComponentVersion's namespace<br />that grants access to this Release's namespace. |  |  |
| `targetNamespace` _string_ | TargetNamespace is the namespace the ComponentVersion gets deployed to. |  |  |
| `values` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Values contains deployment-specific values or configuration for the release.<br />These values override defaults from the component version and are used during deployment. |  |  |
| `failedJobTTL` _integer_ | failedJobTTL is the TTL in seconds after which a failed render job and its secrets are cleaned up.<br />After this duration, the Kubernetes TTL controller will delete the Job and the controller will delete<br />the Secrets (ConfigSecret, AuthSecret). On success, Job and Secrets are deleted immediately.<br />If not set, defaults to 3600 (1 hour). |  |  |


#### ReleaseStatus



ReleaseStatus defines the observed state of a Release.



_Appears in:_
- [Release](#release)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a Release's state. |  |  |
| `renderTaskRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectreference-v1-core)_ | RenderTaskRef is a reference to the RenderTask responsible for this Release. |  |  |
| `chartURL` _string_ | ChartURL represents the URL of where the rendered chart was pushed to. |  |  |




#### RenderTask



RenderTask manages a rendering job



_Appears in:_
- [RenderTaskList](#rendertasklist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RenderTaskSpec](#rendertaskspec)_ |  |  |  |
| `status` _[RenderTaskStatus](#rendertaskstatus)_ |  |  |  |




#### RenderTaskSpec



RenderTaskSpec holds the specification for a RenderTask



_Appears in:_
- [RenderTask](#rendertask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[RendererConfigType](#rendererconfigtype)_ | Type defines the output type of the renderer. |  |  |
| `release` _[ReleaseConfig](#releaseconfig)_ | ReleaseConfig is a config for a release. |  |  |
| `bootstrap` _[BootstrapConfig](#bootstrapconfig)_ | BootstrapConfig is a config for a bootstrap. |  |  |
| `repository` _string_ | Repository is the Repository where the chart will be pushed to (e.g. charts/mychart) |  |  |
| `tag` _string_ | Tag is the Tag of the helm chart to be pushed.<br />Make sure that the tag matches the version in Chart.yaml, otherwise helm<br />will error before pushing. |  |  |
| `baseURL` _string_ | BaseURL is the registry URL to push the rendered chart to (e.g. "registry.example.com:5000"). |  |  |
| `pushSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | PushSecretRef references a Secret in the same namespace with registry credentials<br />for pushing the rendered chart. |  |  |
| `failedJobTTL` _integer_ | failedJobTTL is the TTL in seconds after which a failed render job and its secrets are cleaned up.<br />After this duration, the Kubernetes TTL controller will delete the Job and the controller will delete<br />the Secrets (ConfigSecret, AuthSecret). On success, Job and Secrets are deleted immediately.<br />If not set, defaults to 3600 (1 hour). |  |  |
| `ownerName` _string_ | OwnerName is the name of the resource that created this RenderTask. |  | MinLength: 1 <br /> |
| `ownerNamespace` _string_ | OwnerNamespace is the namespace of the resource that created this RenderTask. |  | MinLength: 1 <br /> |
| `ownerKind` _string_ | OwnerKind is the kind of the resource that created this RenderTask (e.g. Release, Target). |  | MinLength: 1 <br /> |


#### RenderTaskStatus



RenderTaskStatus holds the status of the rendering process



_Appears in:_
- [RenderTask](#rendertask)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a RenderTask's state. |  |  |
| `jobRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectreference-v1-core)_ | JobRef is a reference to the Job that is executing the rendering. |  |  |
| `configSecretRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectreference-v1-core)_ | ConfigSecretRef is a reference to the Secret containing the renderer configuration. |  |  |
| `chartURL` _string_ | ChartURL represents the URL of where the rendered chart was pushed to. |  |  |


#### RendererConfig



RendererConfig defines the configuration for the renderer.



_Appears in:_
- [RenderTaskSpec](#rendertaskspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[RendererConfigType](#rendererconfigtype)_ | Type defines the output type of the renderer. |  |  |
| `release` _[ReleaseConfig](#releaseconfig)_ | ReleaseConfig is a config for a release. |  |  |
| `bootstrap` _[BootstrapConfig](#bootstrapconfig)_ | BootstrapConfig is a config for a bootstrap. |  |  |


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


#### ResourceAccess



ResourceAccess defines how a Resource can be accessed.



_Appears in:_
- [BootstrapInput](#bootstrapinput)
- [ComponentVersionSpec](#componentversionspec)
- [ReleaseInput](#releaseinput)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ | Repository of the Resource. |  |  |
| `insecure` _boolean_ | Insecure switches TLS/HTTPS off if true |  |  |
| `tag` _string_ | Tag of the Resource. |  |  |


#### Target



Target represents a deployment target environment.
It defines the intended state of releases and configuration for a specific deployment target,
such as a cluster or environment.



_Appears in:_
- [TargetList](#targetlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[TargetSpec](#targetspec)_ |  |  |  |
| `status` _[TargetStatus](#targetstatus)_ |  |  |  |




#### TargetSecretReference



TargetSecretReference is a reference to a Secret in a target cluster.



_Appears in:_
- [RegistrySpec](#registryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the Secret. |  |  |
| `namespace` _string_ | Namespace is the namespace of the Secret. |  |  |


#### TargetSpec



TargetSpec defines the desired state of a Target.
It specifies the render registry and configuration for this deployment target.



_Appears in:_
- [Target](#target)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `renderRegistryRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | RenderRegistryRef references the Registry to push rendered desired state to.<br />The referenced Registry must have SolarSecretRef set for rendering to succeed. |  |  |
| `renderRegistryNamespace` _string_ | RenderRegistryNamespace is the namespace of the Registry when it resides in a different<br />namespace than this Target. If empty, the Registry is assumed to be in the same namespace.<br />Cross-namespace references require a ReferenceGrant in the registry's namespace that grants<br />access to this Target's namespace. |  |  |
| `userdata` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Userdata contains arbitrary custom data or configuration specific to this target.<br />This enables target-specific customization and deployment parameters. |  |  |


#### TargetStatus



TargetStatus defines the observed state of a Target.



_Appears in:_
- [Target](#target)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bootstrapVersion` _integer_ | BootstrapVersion is a monotonically increasing counter used as the bootstrap<br />chart version. It is incremented each time the bootstrap chart is re-rendered,<br />e.g. when the set of bound releases changes. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a Target's state. |  |  |


