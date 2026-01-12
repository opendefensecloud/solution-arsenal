# API Reference

## Packages
- [solar.opendefense.cloud/v1alpha1](#solaropendefensecloudv1alpha1)


## solar.opendefense.cloud/v1alpha1

Package v1alpha1 is the v1alpha1 version of the API.



#### Attestation



Attestation represents a security attestation for the component.



_Appears in:_
- [CatalogItemStatus](#catalogitemstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type is the attestation type (e.g., "vulnerability-scan", "stig-compliance", "signature"). |  | MinLength: 1 <br />Required: \{\} <br /> |
| `issuer` _string_ | Issuer identifies who created the attestation. |  | MinLength: 1 <br />Required: \{\} <br /> |
| `reference` _string_ | Reference is a URL or identifier pointing to the attestation data. |  |  |
| `passed` _boolean_ | Passed indicates whether the attestation check passed. |  |  |
| `timestamp` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta)_ | Timestamp is when the attestation was created. |  |  |


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


#### CatalogItemCategory

_Underlying type:_ _string_

CatalogItemCategory defines the type of catalog item.

_Validation:_
- Enum: [Application Operator Addon Library]

_Appears in:_
- [CatalogItemSpec](#catalogitemspec)

| Field | Description |
| --- | --- |
| `Application` | CatalogItemCategoryApplication represents a deployable application.<br /> |
| `Operator` | CatalogItemCategoryOperator represents a Kubernetes operator.<br /> |
| `Addon` | CatalogItemCategoryAddon represents a cluster addon or extension.<br /> |
| `Library` | CatalogItemCategoryLibrary represents a shared library or dependency.<br /> |




#### CatalogItemPhase

_Underlying type:_ _string_

CatalogItemPhase describes the lifecycle phase of a CatalogItem.

_Validation:_
- Enum: [Discovered Validating Available Failed Deprecated]

_Appears in:_
- [CatalogItemStatus](#catalogitemstatus)

| Field | Description |
| --- | --- |
| `Discovered` | CatalogItemPhaseDiscovered indicates the item was found by discovery but not yet validated.<br /> |
| `Validating` | CatalogItemPhaseValidating indicates the item is being validated.<br /> |
| `Available` | CatalogItemPhaseAvailable indicates the item is validated and available for deployment.<br /> |
| `Failed` | CatalogItemPhaseFailed indicates validation or processing has failed.<br /> |
| `Deprecated` | CatalogItemPhaseDeprecated indicates the item is deprecated and should not be used for new deployments.<br /> |


#### CatalogItemSpec



CatalogItemSpec defines the desired state of a CatalogItem.



_Appears in:_
- [CatalogItem](#catalogitem)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `componentName` _string_ | ComponentName is the OCM component name. |  | MinLength: 1 <br />Required: \{\} <br /> |
| `version` _string_ | Version is the semantic version of the component. |  | Pattern: `^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?(\+[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$` <br />Required: \{\} <br /> |
| `repository` _string_ | Repository is the OCI repository URL where the component is stored. |  | MinLength: 1 <br />Required: \{\} <br /> |
| `description` _string_ | Description provides a human-readable description of the catalog item. |  | MaxLength: 4096 <br /> |
| `category` _[CatalogItemCategory](#catalogitemcategory)_ | Category classifies the type of catalog item. |  | Enum: [Application Operator Addon Library] <br /> |
| `maintainers` _[Maintainer](#maintainer) array_ | Maintainers lists the maintainers of this catalog item. |  |  |
| `tags` _string array_ | Tags are labels for searching and filtering catalog items. |  | MaxItems: 20 <br /> |
| `requiredAttestations` _string array_ | RequiredAttestations lists the attestation types required before deployment. |  |  |
| `dependencies` _[ComponentDependency](#componentdependency) array_ | Dependencies lists other OCM components this item depends on. |  |  |
| `minKubernetesVersion` _string_ | MinKubernetesVersion is the minimum Kubernetes version required. |  | Pattern: `^v?[0-9]+\.[0-9]+(\.[0-9]+)?$` <br /> |
| `requiredCapabilities` _string array_ | RequiredCapabilities lists Kubernetes features or CRDs required (e.g., "networking.k8s.io/v1/NetworkPolicy"). |  |  |
| `estimatedResources` _[ResourceRequirements](#resourcerequirements)_ | EstimatedResources provides resource estimates for capacity planning. |  |  |
| `deprecated` _boolean_ | Deprecated marks this catalog item as deprecated. |  |  |
| `deprecationMessage` _string_ | DeprecationMessage provides information about why this item is deprecated and what to use instead. |  | MaxLength: 1024 <br /> |


#### CatalogItemStatus



CatalogItemStatus defines the observed state of a CatalogItem.



_Appears in:_
- [CatalogItem](#catalogitem)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[CatalogItemPhase](#catalogitemphase)_ | Phase is the current lifecycle phase of the catalog item. |  | Enum: [Discovered Validating Available Failed Deprecated] <br /> |
| `validation` _[ValidationStatus](#validationstatus)_ | Validation contains the results of validation checks. |  |  |
| `attestations` _[Attestation](#attestation) array_ | Attestations contains the attestations found for this component. |  |  |
| `lastDiscoveredAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta)_ | LastDiscoveredAt is when this item was last seen by the discovery service. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#condition-v1-meta) array_ | Conditions represent the latest available observations of the catalog item's state. |  |  |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed by the controller. |  |  |


#### ComponentDependency



ComponentDependency describes a dependency on another OCM component.



_Appears in:_
- [CatalogItemSpec](#catalogitemspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `componentName` _string_ | ComponentName is the OCM component name of the dependency. |  | MinLength: 1 <br />Required: \{\} <br /> |
| `versionConstraint` _string_ | VersionConstraint is a semver constraint for acceptable versions (e.g., ">=1.0.0", "^2.0.0"). |  |  |
| `optional` _boolean_ | Optional indicates whether this dependency is optional. |  |  |


#### Maintainer



Maintainer contains information about the maintainer of a catalog item.



_Appears in:_
- [CatalogItemSpec](#catalogitemspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the maintainer's name. |  | MinLength: 1 <br />Required: \{\} <br /> |
| `email` _string_ | Email is the maintainer's email address. |  |  |


#### ResourceRequirements



ResourceRequirements describes the resource requirements for deploying the catalog item.



_Appears in:_
- [CatalogItemSpec](#catalogitemspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cpuCores` _string_ | CPUCores is the estimated CPU cores required. |  | Pattern: `^[0-9]+(\.[0-9]+)?$` <br /> |
| `memoryMB` _string_ | MemoryMB is the estimated memory in megabytes required. |  | Pattern: `^[0-9]+$` <br /> |
| `storageGB` _string_ | StorageGB is the estimated storage in gigabytes required. |  | Pattern: `^[0-9]+(\.[0-9]+)?$` <br /> |


#### ValidationCheck



ValidationCheck represents the result of a single validation check.



_Appears in:_
- [ValidationStatus](#validationstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the validation check. |  | MinLength: 1 <br />Required: \{\} <br /> |
| `passed` _boolean_ | Passed indicates whether the check passed. |  |  |
| `message` _string_ | Message provides additional information about the check result. |  |  |
| `timestamp` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta)_ | Timestamp is when the check was performed. |  |  |


#### ValidationStatus



ValidationStatus contains the results of all validation checks.



_Appears in:_
- [CatalogItemStatus](#catalogitemstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `checks` _[ValidationCheck](#validationcheck) array_ | Checks contains the results of individual validation checks. |  |  |
| `lastValidatedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.34/#time-v1-meta)_ | LastValidatedAt is when validation was last performed. |  |  |


