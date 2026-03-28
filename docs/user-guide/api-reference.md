# API Reference

## Packages
- [solar.opendefense.cloud/v1alpha1](#solaropendefensecloudv1alpha1)


## solar.opendefense.cloud/v1alpha1

Package v1alpha1 is the v1alpha1 version of the API.



#### AuthenticationType

_Underlying type:_ _string_

AuthenticationType



_Appears in:_
- [WebhookAuth](#webhookauth)

| Field | Description |
| --- | --- |
| `Basic` |  |
| `Token` |  |


#### ChartConfig



ChartConfig defines parameters for the rendered chart.



_Appears in:_
- [HydratedTargetConfig](#hydratedtargetconfig)
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



#### Discovery



Discovery represents a configuration for a registry to discover.



_Appears in:_
- [DiscoveryList](#discoverylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[DiscoverySpec](#discoveryspec)_ |  |  |  |
| `status` _[DiscoveryStatus](#discoverystatus)_ |  |  |  |




#### DiscoverySpec



DiscoverySpec defines the desired state of a Discovery.



_Appears in:_
- [Discovery](#discovery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `registry` _[Registry](#registry)_ | Registry specifies the registry that should be scanned by the discovery process. |  |  |
| `webhook` _[Webhook](#webhook)_ | Webhook specifies the configuration for a webhook that is called by the registry on created, updated or deleted images/repositories. |  |  |
| `filter` _[Filter](#filter)_ | Filter specifies the filter that should be applied when scanning for components. If not specified, all components will be scanned. |  | Optional: \{\} <br /> |
| `discoveryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#duration-v1-meta)_ | DiscoveryInterval is the amount of time between two full scans of the registry.<br />Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"<br />May be set to zero to fetch and create it once. Defaults to 24h. | 24h | Optional: \{\} <br /> |
| `disableStartupDiscovery` _boolean_ | DisableStartupDiscovery defines whether the discovery should not be run on startup of the discovery process. If true it will only run on schedule, see .spec.cron. |  |  |


#### DiscoveryStatus



DiscoveryStatus defines the observed state of a Discovery.



_Appears in:_
- [Discovery](#discovery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podGeneration` _integer_ | PodGeneration is the generation of the discovery object at the time the worker was instantiated. |  |  |


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


#### Filter



Filter defines the filter criteria used to determine which components should be scanned.



_Appears in:_
- [DiscoverySpec](#discoveryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repositoryPatterns` _string array_ | RepositoryPatterns defines which repositories should be scanned for components. The default value is empty, which means that all repositories will be scanned.<br />Wildcards are supported, e.g. "foo-*" or "*-dev". |  |  |


#### HydratedTarget



HydratedTarget represents a fully resolved and configured deployment target.
It resolves the implicit matching of profiles to produce a concrete set of releases and profiles.



_Appears in:_
- [HydratedTargetList](#hydratedtargetlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[HydratedTargetSpec](#hydratedtargetspec)_ |  |  |  |
| `status` _[HydratedTargetStatus](#hydratedtargetstatus)_ |  |  |  |


#### HydratedTargetConfig



HydratedTargetConfig defines the render config for a hydrated-target.



_Appears in:_
- [RenderTaskSpec](#rendertaskspec)
- [RendererConfig](#rendererconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `chart` _[ChartConfig](#chartconfig)_ | Chart is the ChartConfig for the rendered chart. |  |  |
| `input` _[HydratedTargetInput](#hydratedtargetinput)_ | Input is the input of the hydrated-target. |  |  |


#### HydratedTargetInput



HydratedTargetInput defines the inputs to render a hydrated-target.



_Appears in:_
- [HydratedTargetConfig](#hydratedtargetconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `releases` _object (keys:string, values:[ResourceAccess](#resourceaccess))_ |  |  |  |
| `userdata` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Userdata is additional data to be rendered into the hydrated-target chart values. |  |  |




#### HydratedTargetSpec



HydratedTargetSpec defines the desired state of a HydratedTarget.
It contains the concrete releases, profiles, and deployment configuration for a target environment.



_Appears in:_
- [HydratedTarget](#hydratedtarget)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `releases` _object (keys:string, values:[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core))_ | Releases is a map of release names to their corresponding Release object references.<br />Each entry represents a component release that will be deployed to the target. |  |  |
| `profiles` _object (keys:string, values:[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core))_ | Profiles is a map of profile names to their corresponding Profile object references.<br />It points to profiles that match the target, e.g. through the label selector of the Profile |  |  |
| `userdata` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Userdata contains arbitrary custom data or configuration for the target deployment.<br />This allows providing target-specific parameters or settings. |  |  |


#### HydratedTargetStatus



HydratedTargetStatus defines the observed state of a HydratedTarget.



_Appears in:_
- [HydratedTarget](#hydratedtarget)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of a HydratedTarget's state. |  |  |
| `renderTaskRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectreference-v1-core)_ | RenderTaskRef is a reference to the RenderTask responsible for this HydratedTarget. |  |  |


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




#### Registry



Registry defines the configuration for a registry.



_Appears in:_
- [DiscoverySpec](#discoveryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `registryURL` _string_ | RegistryURL defines the URL which is used to connect to the registry. |  |  |
| `secretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | SecretRef specifies the secret containing the relevant credentials for the registry that should be used during discovery. |  |  |
| `caConfigMapRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | CAConfigMapRef contains CA bundle for registry connections (e.g., trust-manager's root-bundle). Key is expected to be "trust-bundle.pem". |  |  |
| `plainHTTP` _boolean_ | PlainHTTP defines whether the registry should be accessed via plain HTTP instead of HTTPS. |  |  |


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
| `values` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Values contains deployment-specific values or configuration for the release.<br />These values override defaults from the component version and are used during deployment. |  |  |
| `failedJobTTL` _integer_ | failedJobTTL is the TTL in seconds for the Kubernetes TTL controller to clean up a failed render job.<br />After this duration, the Kubernetes TTL controller will delete the Job.<br />Secrets (ConfigSecret, AuthSecret) are cleaned up separately by the controller<br />when the parent Release is deleted or when the job succeeds.<br />If not set, defaults to 3600 (1 hour). |  |  |


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
| `hydrated-target` _[HydratedTargetConfig](#hydratedtargetconfig)_ | HydratedTargetConfig is a config for a hydrated-target. |  |  |
| `repository` _string_ | Repository is the Repository where the chart will be pushed to (e.g. charts/mychart)<br />Keep in mind that the repository gets automatically prefixed with the<br />registry by the rendertask-controller. |  |  |
| `tag` _string_ | Tag is the Tag of the helm chart to be pushed.<br />Make sure that the tag matches the version in Chart.yaml, otherwise helm<br />will error before pushing. |  |  |
| `failedJobTTL` _integer_ | failedJobTTL is the TTL in seconds for the Kubernetes TTL controller to clean up a failed render job.<br />After this duration, the Kubernetes TTL controller will delete the Job.<br />Secrets (ConfigSecret, AuthSecret) are cleaned up separately by the controller<br />when the parent Release is deleted or when the job succeeds.<br />If not set, defaults to 3600 (1 hour). |  |  |


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
| `hydrated-target` _[HydratedTargetConfig](#hydratedtargetconfig)_ | HydratedTargetConfig is a config for a hydrated-target. |  |  |


#### RendererConfigType

_Underlying type:_ _string_

RendererConfigType is the output type of the renderer.



_Appears in:_
- [RenderTaskSpec](#rendertaskspec)
- [RendererConfig](#rendererconfig)

| Field | Description |
| --- | --- |
| `hydrated-target` |  |
| `release` |  |
| `profile` |  |


#### ResourceAccess



ResourceAccess defines how a Resource can be accessed.



_Appears in:_
- [ComponentVersionSpec](#componentversionspec)
- [HydratedTargetInput](#hydratedtargetinput)
- [ReleaseInput](#releaseinput)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ | Repository of the Resource. |  |  |
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




#### TargetSpec



TargetSpec defines the desired state of a Target.
It specifies the releases and configuration intended for this deployment target.



_Appears in:_
- [Target](#target)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `releases` _object (keys:string, values:[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core))_ | Releases is a map of release names to their corresponding Release object references.<br />Each entry represents a component release intended for deployment on this target. |  |  |
| `userdata` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Userdata contains arbitrary custom data or configuration specific to this target.<br />This enables target-specific customization and deployment parameters. |  |  |


#### TargetStatus



TargetStatus defines the observed state of a Target.



_Appears in:_
- [Target](#target)



#### Webhook



Webhook represents the configuration for a webhook.



_Appears in:_
- [DiscoverySpec](#discoveryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `flavor` _string_ | Flavor is the webhook implementation to use. |  | Pattern: `^(@(zot)$` <br /> |
| `path` _string_ | Path is where the webhook should listen. |  |  |
| `auth` _[WebhookAuth](#webhookauth)_ | Auth is the authentication information to use with the webhook. |  |  |


#### WebhookAuth







_Appears in:_
- [Webhook](#webhook)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[AuthenticationType](#authenticationtype)_ | Type represents the type of authentication to use. Currently, only "token" is supported. |  |  |
| `authSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | AuthSecretRef is the reference to the secret which contains the authentication information for the webhook. |  |  |


