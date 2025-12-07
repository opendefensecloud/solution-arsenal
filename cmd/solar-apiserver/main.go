// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"go.opendefense.cloud/kit/apiserver"
	"go.opendefense.cloud/solar/api/solar"
	"go.opendefense.cloud/solar/api/solar/install"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/client-go/openapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	componentName = "solar"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	install.Install(scheme)

	// we need to add the options to empty v1
	// TODO: fix the server code to avoid this
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})

	// TODO: keep the generic API server from wanting this
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}
func main() {
	code := apiserver.NewBuilder(scheme).
		WithComponentName(componentName).
		WithOpenAPIDefinitions(componentName, "v0.1.0", openapi.GetOpenAPIDefinitions).
		With(apiserver.Resource(&solar.CatalogItem{}, solarv1alpha1.SchemeGroupVersion)).
		Execute()
	os.Exit(code)
}
