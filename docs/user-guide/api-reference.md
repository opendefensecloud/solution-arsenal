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


#### Cron



Cron represents a cron schedule.



_Appears in:_
- [DiscoverySpec](#discoveryspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `timezone` _string_ | Timezone is the timezone against which the cron schedule will be calculated, e.g. "Asia/Tokyo". Default is machine's local time. |  |  |
| `startingDeadlineSeconds` _integer_ | StartingDeadlineSeconds is the K8s-style deadline that will limit the time a schedule will be run after its<br />original scheduled time if it is missed. |  | Minimum: 0 <br /> |
| `schedules` _string array_ | Schedules is a list of schedules to run in Cron format |  | MinItems: 1 <br />items:Pattern: ^(@(yearly\|annually\|monthly\|weekly\|daily\|midnight\|hourly)\|@every\s+([0-9]+(ns\|us\|µs\|ms\|s\|m\|h))+\|([0-9*,/?-]+\s+)\{4\}[0-9*,/?-]+)$ <br /> |


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
| `remoteURL` _string_ | RemoteURL defines the URL which is used to connect to the registry. |  |  |
| `discoverySecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | SecretRef specifies the secret containing the relevant credentials for the registry that should be used during discovery. |  |  |
| `releaseSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#localobjectreference-v1-core)_ | SecretRef specifies the secret containing the relevant credentials for the registry that should be used when a discovered component is part of a release. If not specified uses .spec.discoverySecretRef. |  |  |
| `cron` _[Cron](#cron)_ | Cron specifies options which determine when the discover process should run for the given registry. |  |  |
| `disableStartupDiscovery` _boolean_ | DisableStartupDiscovery defines whether the discovery should not be run on startup of the discovery process. If true it will only run on schedule, see .spec.cron. |  |  |


#### DiscoveryStatus



DiscoveryStatus defines the observed state of a Discovery.



_Appears in:_
- [Discovery](#discovery)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _string_ | Phase tracks the phase of the discovery process |  |  |
| `message` _string_ | A human readable message describing the current status of the discovery process. |  |  |
| `lastDiscovery` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta)_ | LastDiscovery is the last time the discovery has run |  |  |


