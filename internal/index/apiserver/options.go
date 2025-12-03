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

package apiserver

import (
	"fmt"
	"net"

	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/options"

	solarv1alpha1 "github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1"
	generatedopenapi "github.com/opendefensecloud/solution-arsenal/pkg/generated/openapi"
)

// Options contains the configuration options for the solar-index API server.
type Options struct {
	// RecommendedOptions contains the recommended options for running an API server.
	RecommendedOptions *options.RecommendedOptions

	// StdOut and StdErr are used for output.
	StdOut, StdErr interface{}
}

// NewOptions creates a new Options with default values.
func NewOptions() *Options {
	o := &Options{
		RecommendedOptions: options.NewRecommendedOptions(
			"",
			Codecs.LegacyCodec(solarv1alpha1.GroupVersion),
		),
	}

	// Disable etcd by default for in-memory storage during development.
	// In production, etcd should be enabled.
	o.RecommendedOptions.Etcd = nil

	return o
}

// AddFlags adds flags for the options to the specified FlagSet.
func (o *Options) AddFlags(fs *pflag.FlagSet) {
	o.RecommendedOptions.AddFlags(fs)
}

// Validate validates the options.
func (o *Options) Validate() []error {
	var errors []error
	errors = append(errors, o.RecommendedOptions.Validate()...)
	return errors
}

// Complete fills in default values for options.
func (o *Options) Complete() error {
	return nil
}

// Config returns a Config for running the solar-index API server.
func (o *Options) Config() (*Config, error) {
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts(
		"localhost",
		nil,
		[]net.IP{net.ParseIP("127.0.0.1")},
	); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %w", err)
	}

	serverConfig := server.NewRecommendedConfig(Codecs)
	serverConfig.OpenAPIConfig = server.DefaultOpenAPIConfig(
		generatedopenapi.GetOpenAPIDefinitions,
		openapi.NewDefinitionNamer(Scheme),
	)
	serverConfig.OpenAPIConfig.Info.Title = "Solar Index API"
	serverConfig.OpenAPIConfig.Info.Version = "v1alpha1"

	serverConfig.OpenAPIV3Config = server.DefaultOpenAPIV3Config(
		generatedopenapi.GetOpenAPIDefinitions,
		openapi.NewDefinitionNamer(Scheme),
	)
	serverConfig.OpenAPIV3Config.Info.Title = "Solar Index API"
	serverConfig.OpenAPIV3Config.Info.Version = "v1alpha1"

	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, fmt.Errorf("error applying recommended options: %w", err)
	}

	return &Config{
		GenericConfig: serverConfig,
	}, nil
}

// Config contains the configuration for the solar-index API server.
type Config struct {
	GenericConfig *server.RecommendedConfig
}

// Complete fills in default values for the configuration.
func (c *Config) Complete() CompletedConfig {
	return CompletedConfig{
		GenericConfig: c.GenericConfig.Complete(),
	}
}

// CompletedConfig contains the completed configuration for the solar-index API server.
type CompletedConfig struct {
	GenericConfig server.CompletedConfig
}

// Scheme is the runtime scheme containing all API types.
var Scheme = runtime.NewScheme()

// Codecs provides encoders and decoders for the API types.
var Codecs = serializer.NewCodecFactory(Scheme)

func init() {
	// Add solar API types to the scheme.
	if err := solarv1alpha1.AddToScheme(Scheme); err != nil {
		panic(err)
	}

	// Add the default Kubernetes types.
	// We need metav1 for list operations.
	// metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})
}
