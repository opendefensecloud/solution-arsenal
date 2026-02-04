// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"go.opendefense.cloud/kit/apiserver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"go.opendefense.cloud/solar/api/solar"
	"go.opendefense.cloud/solar/api/solar/install"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/client-go/openapi"
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
		With(apiserver.Resource(&solar.Discovery{}, solarv1alpha1.SchemeGroupVersion)).
		With(apiserver.Resource(&solar.Component{}, solarv1alpha1.SchemeGroupVersion)).
		With(apiserver.Resource(&solar.ComponentVersion{}, solarv1alpha1.SchemeGroupVersion)).
		With(apiserver.Resource(&solar.Release{}, solarv1alpha1.SchemeGroupVersion)).
		With(apiserver.Resource(&solar.Target{}, solarv1alpha1.SchemeGroupVersion)).
		With(apiserver.Resource(&solar.HydratedTarget{}, solarv1alpha1.SchemeGroupVersion)).
		Execute()
	os.Exit(code)
}
