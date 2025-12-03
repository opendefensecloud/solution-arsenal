/*
Copyright 2024 Open Defense Cloud Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package openapi contains generated OpenAPI definitions for the Solar API.
package openapi

import (
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

// GetOpenAPIDefinitions returns the OpenAPI definitions for the Solar API.
func GetOpenAPIDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	return map[string]common.OpenAPIDefinition{
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.CatalogItem":              catalogItemOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.CatalogItemList":          catalogItemListOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.CatalogItemSpec":          catalogItemSpecOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.CatalogItemStatus":        catalogItemStatusOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ClusterCatalogItem":       clusterCatalogItemOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ClusterCatalogItemList":   clusterCatalogItemListOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ClusterRegistration":      clusterRegistrationOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ClusterRegistrationList":  clusterRegistrationListOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ClusterRegistrationSpec":  clusterRegistrationSpecOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ClusterRegistrationStatus": clusterRegistrationStatusOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.Release":                  releaseOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ReleaseList":              releaseListOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ReleaseSpec":              releaseSpecOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ReleaseStatus":            releaseStatusOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.Sync":                     syncOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.SyncList":                 syncListOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.SyncSpec":                 syncSpecOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.SyncStatus":               syncStatusOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ObjectReference":          objectReferenceOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ComponentReference":       componentReferenceOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.AgentConfiguration":       agentConfigurationOpenAPI(ref),
		"github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.SyncFilter":               syncFilterOpenAPI(ref),
	}
}

func catalogItemOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "CatalogItem represents an OCM package available in the catalog. CatalogItems are namespaced and represent solutions available within a tenant.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Description: "APIVersion defines the versioned schema of this representation of an object.",
							Type:        []string{"string"},
						},
					},
					"kind": {
						SchemaProps: spec.SchemaProps{
							Description: "Kind is a string value representing the REST resource this object represents.",
							Type:        []string{"string"},
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Description: "Standard object's metadata.",
							Ref:         ref("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Description: "Spec defines the desired state of the CatalogItem.",
							Ref:         ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.CatalogItemSpec"),
						},
					},
					"status": {
						SchemaProps: spec.SchemaProps{
							Description: "Status defines the observed state of the CatalogItem.",
							Ref:         ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.CatalogItemStatus"),
						},
					},
				},
				Required: []string{"spec"},
			},
		},
	}
}

func catalogItemListOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "CatalogItemList contains a list of CatalogItem.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"kind": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ListMeta"),
						},
					},
					"items": {
						SchemaProps: spec.SchemaProps{
							Description: "Items is the list of CatalogItems.",
							Type:        []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.CatalogItem"),
									},
								},
							},
						},
					},
				},
				Required: []string{"items"},
			},
		},
	}
}

func catalogItemSpecOpenAPI(_ common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "CatalogItemSpec defines the desired state of a CatalogItem.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"componentName": {
						SchemaProps: spec.SchemaProps{
							Description: "ComponentName is the OCM component name.",
							Type:        []string{"string"},
						},
					},
					"version": {
						SchemaProps: spec.SchemaProps{
							Description: "Version is the semantic version of the component.",
							Type:        []string{"string"},
						},
					},
					"repository": {
						SchemaProps: spec.SchemaProps{
							Description: "Repository is the OCI repository URL where the component is stored.",
							Type:        []string{"string"},
						},
					},
					"description": {
						SchemaProps: spec.SchemaProps{
							Description: "Description provides a human-readable description of the catalog item.",
							Type:        []string{"string"},
						},
					},
					"labels": {
						SchemaProps: spec.SchemaProps{
							Description: "Labels for categorization and filtering.",
							Type:        []string{"object"},
							AdditionalProperties: &spec.SchemaOrBool{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"string"},
									},
								},
							},
						},
					},
					"dependencies": {
						SchemaProps: spec.SchemaProps{
							Description: "Dependencies lists other components this catalog item depends on.",
							Type:        []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"object"},
									},
								},
							},
						},
					},
				},
				Required: []string{"componentName", "version", "repository"},
			},
		},
	}
}

func catalogItemStatusOpenAPI(_ common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "CatalogItemStatus defines the observed state of a CatalogItem.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"phase": {
						SchemaProps: spec.SchemaProps{
							Description: "Phase indicates the current state of the catalog item.",
							Type:        []string{"string"},
							Enum:        []interface{}{"Available", "Unavailable", "Deprecated"},
						},
					},
					"conditions": {
						SchemaProps: spec.SchemaProps{
							Description: "Conditions represent the latest available observations of the catalog item's state.",
							Type:        []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"object"},
									},
								},
							},
						},
					},
					"lastScanned": {
						SchemaProps: spec.SchemaProps{
							Description: "LastScanned is the timestamp of the last discovery scan.",
							Type:        []string{"string"},
							Format:      "date-time",
						},
					},
				},
			},
		},
	}
}

func clusterCatalogItemOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ClusterCatalogItem is a cluster-scoped variant of CatalogItem. ClusterCatalogItems are available to all tenants in the cluster.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"kind": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.CatalogItemSpec"),
						},
					},
					"status": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.CatalogItemStatus"),
						},
					},
				},
			},
		},
	}
}

func clusterCatalogItemListOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ClusterCatalogItemList contains a list of ClusterCatalogItem.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"kind": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ListMeta"),
						},
					},
					"items": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ClusterCatalogItem"),
									},
								},
							},
						},
					},
				},
				Required: []string{"items"},
			},
		},
	}
}

func clusterRegistrationOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ClusterRegistration represents a Kubernetes cluster registered with SolAr.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"kind": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ClusterRegistrationSpec"),
						},
					},
					"status": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ClusterRegistrationStatus"),
						},
					},
				},
			},
		},
	}
}

func clusterRegistrationListOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ClusterRegistrationList contains a list of ClusterRegistration.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"kind": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ListMeta"),
						},
					},
					"items": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ClusterRegistration"),
									},
								},
							},
						},
					},
				},
				Required: []string{"items"},
			},
		},
	}
}

func clusterRegistrationSpecOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ClusterRegistrationSpec defines the desired state of a ClusterRegistration.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"displayName": {
						SchemaProps: spec.SchemaProps{
							Description: "DisplayName is a human-readable name for the cluster.",
							Type:        []string{"string"},
						},
					},
					"description": {
						SchemaProps: spec.SchemaProps{
							Description: "Description provides additional context about the cluster.",
							Type:        []string{"string"},
						},
					},
					"labels": {
						SchemaProps: spec.SchemaProps{
							Description: "Labels for cluster categorization and targeting.",
							Type:        []string{"object"},
							AdditionalProperties: &spec.SchemaOrBool{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"string"},
									},
								},
							},
						},
					},
					"agentConfig": {
						SchemaProps: spec.SchemaProps{
							Description: "AgentConfig holds configuration for the solar-agent deployed to this cluster.",
							Ref:         ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.AgentConfiguration"),
						},
					},
				},
				Required: []string{"displayName"},
			},
		},
	}
}

func clusterRegistrationStatusOpenAPI(_ common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ClusterRegistrationStatus defines the observed state of a ClusterRegistration.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"phase": {
						SchemaProps: spec.SchemaProps{
							Description: "Phase indicates the current state of the cluster registration.",
							Type:        []string{"string"},
							Enum:        []interface{}{"Pending", "Connecting", "Ready", "NotReady", "Unreachable"},
						},
					},
					"conditions": {
						SchemaProps: spec.SchemaProps{
							Description: "Conditions represent the latest available observations.",
							Type:        []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"object"},
									},
								},
							},
						},
					},
					"agentVersion": {
						SchemaProps: spec.SchemaProps{
							Description: "AgentVersion is the version of the solar-agent running in the cluster.",
							Type:        []string{"string"},
						},
					},
					"kubernetesVersion": {
						SchemaProps: spec.SchemaProps{
							Description: "KubernetesVersion is the version of Kubernetes in the cluster.",
							Type:        []string{"string"},
						},
					},
					"lastHeartbeat": {
						SchemaProps: spec.SchemaProps{
							Description: "LastHeartbeat is the timestamp of the last agent heartbeat.",
							Type:        []string{"string"},
							Format:      "date-time",
						},
					},
					"installedReleases": {
						SchemaProps: spec.SchemaProps{
							Description: "InstalledReleases lists releases currently installed on this cluster.",
							Type:        []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"object"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func releaseOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "Release represents a deployment of a CatalogItem to a cluster.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"kind": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ReleaseSpec"),
						},
					},
					"status": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ReleaseStatus"),
						},
					},
				},
			},
		},
	}
}

func releaseListOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ReleaseList contains a list of Release.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"kind": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ListMeta"),
						},
					},
					"items": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.Release"),
									},
								},
							},
						},
					},
				},
				Required: []string{"items"},
			},
		},
	}
}

func releaseSpecOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ReleaseSpec defines the desired state of a Release.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"catalogItemRef": {
						SchemaProps: spec.SchemaProps{
							Description: "CatalogItemRef references the catalog item to deploy.",
							Ref:         ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ObjectReference"),
						},
					},
					"targetClusterRef": {
						SchemaProps: spec.SchemaProps{
							Description: "TargetClusterRef references the target cluster for deployment.",
							Ref:         ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ObjectReference"),
						},
					},
					"values": {
						SchemaProps: spec.SchemaProps{
							Description: "Values are the configuration values for the release.",
							Type:        []string{"object"},
						},
					},
					"suspend": {
						SchemaProps: spec.SchemaProps{
							Description: "Suspend prevents reconciliation when set to true.",
							Type:        []string{"boolean"},
						},
					},
				},
				Required: []string{"catalogItemRef", "targetClusterRef"},
			},
		},
	}
}

func releaseStatusOpenAPI(_ common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ReleaseStatus defines the observed state of a Release.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"phase": {
						SchemaProps: spec.SchemaProps{
							Description: "Phase indicates the current state of the release.",
							Type:        []string{"string"},
							Enum:        []interface{}{"Pending", "Rendering", "Deploying", "Deployed", "Failed", "Suspended"},
						},
					},
					"conditions": {
						SchemaProps: spec.SchemaProps{
							Description: "Conditions represent the latest available observations.",
							Type:        []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"object"},
									},
								},
							},
						},
					},
					"appliedVersion": {
						SchemaProps: spec.SchemaProps{
							Description: "AppliedVersion is the version currently applied to the cluster.",
							Type:        []string{"string"},
						},
					},
					"lastAppliedTime": {
						SchemaProps: spec.SchemaProps{
							Description: "LastAppliedTime is the timestamp of the last successful apply.",
							Type:        []string{"string"},
							Format:      "date-time",
						},
					},
					"observedGeneration": {
						SchemaProps: spec.SchemaProps{
							Description: "ObservedGeneration is the last observed generation of the release.",
							Type:        []string{"integer"},
							Format:      "int64",
						},
					},
				},
			},
		},
	}
}

func syncOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "Sync represents a catalog chaining configuration.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"kind": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.SyncSpec"),
						},
					},
					"status": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.SyncStatus"),
						},
					},
				},
			},
		},
	}
}

func syncListOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "SyncList contains a list of Sync.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"kind": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"string"},
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ListMeta"),
						},
					},
					"items": {
						SchemaProps: spec.SchemaProps{
							Type: []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Ref: ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.Sync"),
									},
								},
							},
						},
					},
				},
				Required: []string{"items"},
			},
		},
	}
}

func syncSpecOpenAPI(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "SyncSpec defines the desired state of a Sync.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"sourceRef": {
						SchemaProps: spec.SchemaProps{
							Description: "SourceRef references the source catalog item or pattern.",
							Ref:         ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.ObjectReference"),
						},
					},
					"destinationRegistry": {
						SchemaProps: spec.SchemaProps{
							Description: "DestinationRegistry is the destination OCI registry URL.",
							Type:        []string{"string"},
						},
					},
					"filter": {
						SchemaProps: spec.SchemaProps{
							Description: "Filter defines rules for filtering which items to sync.",
							Ref:         ref("github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1.SyncFilter"),
						},
					},
				},
				Required: []string{"sourceRef", "destinationRegistry"},
			},
		},
	}
}

func syncStatusOpenAPI(_ common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "SyncStatus defines the observed state of a Sync.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"phase": {
						SchemaProps: spec.SchemaProps{
							Description: "Phase indicates the current state of the sync.",
							Type:        []string{"string"},
							Enum:        []interface{}{"Pending", "Syncing", "Synced", "Failed"},
						},
					},
					"conditions": {
						SchemaProps: spec.SchemaProps{
							Description: "Conditions represent the latest available observations.",
							Type:        []string{"array"},
							Items: &spec.SchemaOrArray{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"object"},
									},
								},
							},
						},
					},
					"lastSyncTime": {
						SchemaProps: spec.SchemaProps{
							Description: "LastSyncTime is the timestamp of the last successful sync.",
							Type:        []string{"string"},
							Format:      "date-time",
						},
					},
					"syncedItems": {
						SchemaProps: spec.SchemaProps{
							Description: "SyncedItems is the count of items synced in the last operation.",
							Type:        []string{"integer"},
							Format:      "int32",
						},
					},
				},
			},
		},
	}
}

func objectReferenceOpenAPI(_ common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ObjectReference contains enough information to let you locate the referenced object.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"name": {
						SchemaProps: spec.SchemaProps{
							Description: "Name of the referent.",
							Type:        []string{"string"},
						},
					},
					"namespace": {
						SchemaProps: spec.SchemaProps{
							Description: "Namespace of the referent. If not specified, the local namespace is assumed.",
							Type:        []string{"string"},
						},
					},
				},
				Required: []string{"name"},
			},
		},
	}
}

func componentReferenceOpenAPI(_ common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "ComponentReference references an OCM component.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"name": {
						SchemaProps: spec.SchemaProps{
							Description: "Name is the OCM component name.",
							Type:        []string{"string"},
						},
					},
					"version": {
						SchemaProps: spec.SchemaProps{
							Description: "Version is the semantic version of the component.",
							Type:        []string{"string"},
						},
					},
				},
				Required: []string{"name"},
			},
		},
	}
}

func agentConfigurationOpenAPI(_ common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "AgentConfiguration defines the configuration for a solar-agent.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"syncEnabled": {
						SchemaProps: spec.SchemaProps{
							Description: "SyncEnabled enables catalog chaining via Sync resources.",
							Type:        []string{"boolean"},
						},
					},
					"arcEndpoint": {
						SchemaProps: spec.SchemaProps{
							Description: "ARCEndpoint is the ARC endpoint for catalog chaining.",
							Type:        []string{"string"},
						},
					},
				},
			},
		},
	}
}

func syncFilterOpenAPI(_ common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "SyncFilter defines filtering rules for sync operations.",
				Type:        []string{"object"},
				Properties: map[string]spec.Schema{
					"includeLabels": {
						SchemaProps: spec.SchemaProps{
							Description: "IncludeLabels specifies labels that items must have to be included.",
							Type:        []string{"object"},
							AdditionalProperties: &spec.SchemaOrBool{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"string"},
									},
								},
							},
						},
					},
					"excludeLabels": {
						SchemaProps: spec.SchemaProps{
							Description: "ExcludeLabels specifies labels that exclude items from sync.",
							Type:        []string{"object"},
							AdditionalProperties: &spec.SchemaOrBool{
								Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"string"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
