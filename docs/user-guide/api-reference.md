# API Reference

## Packages
- [solar.opendefense.cloud/v1alpha1](#solaropendefensecloudv1alpha1)


## solar.opendefense.cloud/v1alpha1

Package v1alpha1 is the v1alpha1 version of the API.



#### CatalogItem



CatalogItem represents an OCM component available in the solution catalog.



_Appears in:_
- [CatalogItemList](#catalogitemlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[CatalogItemSpec](#catalogitemspec)_ |  |  |  |
| `status` _[CatalogItemStatus](#catalogitemstatus)_ |  |  |  |




#### CatalogItemSpec



CatalogItemSpec defines the desired state of a CatalogItem.



_Appears in:_
- [CatalogItem](#catalogitem)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ | Repository is the OCI repository URL where the component is stored. |  |  |
| `versions` _[CatalogItemVersionSpec](#catalogitemversionspec) array_ | Versions lists the available versions of this component. |  |  |
| `provider` _string_ | Provider is the provider or vendor of the catalog item. |  |  |
| `creationTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta)_ | CreationTime is the creation time of component version |  |  |


#### CatalogItemStatus



CatalogItemStatus defines the observed state of a CatalogItem.



_Appears in:_
- [CatalogItem](#catalogitem)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `lastDiscoveredAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta)_ | LastDiscoveredAt is when this item was last seen by the discovery service. |  |  |


#### CatalogItemVersionSpec







_Appears in:_
- [CatalogItemSpec](#catalogitemspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the semantic version of the component. |  |  |
| `digest` _string_ | Digest is the OCI digest of the component version. |  |  |


#### Discovery



Discovery represents represents a configuration for a registry to discover.



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


#### DiscoveryStatus



DiscoveryStatus defines the observed state of a Discovery.



_Appears in:_
- [Discovery](#discovery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podDiscoveryVersion` _string_ | PodDiscoveryVersion is the version of the discovery object at the time the worker was instantiated. |  |  |


#### Registry







_Appears in:_
- [DiscoverySpec](#discoveryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `registryURL` _string_ | RegistryURL defines the URL which is used to connect to the registry. |  |  |
| `repositoryFilter` _string array_ | RepositoryFilter defines which repositories should be scanned for components. The default value is empty, which means that all repositories will be scanned.<br />Wildcards are supported, e.g. "foo-*" or "*-dev". |  | Optional: \{\} <br /> |
| `discoverySecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | SecretRef specifies the secret containing the relevant credentials for the registry that should be used during discovery. |  |  |
| `releaseSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | SecretRef specifies the secret containing the relevant credentials for the registry that should be used when a discovered component is part of a release. If not specified uses .spec.discoverySecretRef. |  |  |
| `discoveryInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#duration-v1-meta)_ | DiscoveryInterval is the amount of time between two full scans of the registry.<br />Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"<br />May be set to zero to fetch and create it once. Defaults to 24h. | 24h | Optional: \{\} <br /> |
| `disableStartupDiscovery` _boolean_ | DisableStartupDiscovery defines whether the discovery should not be run on startup of the discovery process. If true it will only run on schedule, see .spec.cron. |  |  |


#### Webhook



Webhook represents the configuration for a webhook.



_Appears in:_
- [DiscoverySpec](#discoveryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `flavor` _string_ | Flavor is the webhook implementation to use. |  | Pattern: `^(@(zot)$` <br /> |
| `path` _string_ | Path is where the webhook should listen. |  |  |
| `authTokenSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | AuthTokenSecretRef is the reference to the secret which contains the authentication token for the webhook. |  |  |


