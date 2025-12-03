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

// Package client provides utilities for interacting with the Solar API.
package client

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1"
)

var (
	// Scheme is the runtime scheme containing all Solar API types.
	Scheme = runtime.NewScheme()
)

func init() {
	// Add Kubernetes core types.
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	// Add Solar API types.
	utilruntime.Must(solarv1alpha1.AddToScheme(Scheme))
}

// NewClient creates a new controller-runtime client configured with the Solar scheme.
func NewClient(config *rest.Config, options client.Options) (client.Client, error) {
	if options.Scheme == nil {
		options.Scheme = Scheme
	}
	return client.New(config, options)
}

// NewClientWithWatch creates a new controller-runtime client with watch support.
func NewClientWithWatch(config *rest.Config, options client.Options) (client.WithWatch, error) {
	if options.Scheme == nil {
		options.Scheme = Scheme
	}
	return client.NewWithWatch(config, options)
}
