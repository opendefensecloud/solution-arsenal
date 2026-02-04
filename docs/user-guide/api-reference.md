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
It contains metadata about an OCM component including its repository location,
type classification, and the provider.



_Appears in:_
- [Component](#component)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ | Repository is the OCI repository URL where the component is stored. |  |  |
| `type` _string_ | Type defines what type of Component this is. |  |  |
| `provider` _string_ | Provider identifies the provider or vendor of this component. |  |  |


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
| `componentRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ |  |  |  |
| `tag` _string_ |  |  |  |
| `resources` _object (keys:string, values:[ResourceAccess](#resourceaccess))_ |  |  |  |
| `helm` _[ResourceAccess](#resourceaccess)_ |  |  |  |
| `kro` _[ResourceAccess](#resourceaccess)_ |  |  |  |


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




#### HydratedTargetSpec



HydratedTargetSpec defines the desired state of a HydratedTarget.
It contains the concrete releases and deployment configuration for a target environment.



_Appears in:_
- [HydratedTarget](#hydratedtarget)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `releases` _object (keys:string, values:[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core))_ | Releases is a map of release names to their corresponding Release object references.<br />Each entry represents a component release that will be deployed to the target. |  |  |
| `userdata` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Userdata contains arbitrary custom data or configuration for the target deployment.<br />This allows providing target-specific parameters or settings. |  |  |


#### HydratedTargetStatus



HydratedTargetStatus defines the observed state of a HydratedTarget.



_Appears in:_
- [HydratedTarget](#hydratedtarget)



#### Registry



Registry defines the configuration for a registry.



_Appears in:_
- [DiscoverySpec](#discoveryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `registryURL` _string_ | RegistryURL defines the URL which is used to connect to the registry. |  |  |
| `discoverySecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | SecretRef specifies the secret containing the relevant credentials for the registry that should be used during discovery. |  |  |
| `releaseSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | SecretRef specifies the secret containing the relevant credentials for the registry that should be used when a discovered component is part of a release. If not specified uses .spec.discoverySecretRef. |  |  |


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




#### ReleaseSpec



ReleaseSpec defines the desired state of a Release.
It specifies which component version to release and its deployment configuration.



_Appears in:_
- [Release](#release)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `componentRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | ComponentVersionRef is a reference to the ComponentVersion to be released.<br />It points to the specific version of a component that this release is based on. |  |  |
| `values` _[RawExtension](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#rawextension-runtime-pkg)_ | Values contains deployment-specific values or configuration for the release.<br />These values override defaults from the component version and are used during deployment. |  |  |


#### ReleaseStatus



ReleaseStatus defines the observed state of a Release.



_Appears in:_
- [Release](#release)



#### ResourceAccess







_Appears in:_
- [ComponentVersionSpec](#componentversionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ |  |  |  |
| `tag` _string_ |  |  |  |


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


