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


