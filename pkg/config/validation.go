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

package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}

	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// HasErrors returns true if there are any validation errors.
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validator provides configuration validation.
type Validator struct {
	errors ValidationErrors
}

// NewValidator creates a new Validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Required validates that a string field is not empty.
func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "is required",
		})
	}
	return v
}

// MinLength validates that a string has at least the minimum length.
func (v *Validator) MinLength(field, value string, min int) *Validator {
	if len(value) < min {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("must be at least %d characters", min),
		})
	}
	return v
}

// MaxLength validates that a string has at most the maximum length.
func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if len(value) > max {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("must be at most %d characters", max),
		})
	}
	return v
}

// InRange validates that an integer is within the specified range.
func (v *Validator) InRange(field string, value, min, max int) *Validator {
	if value < min || value > max {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("must be between %d and %d", min, max),
		})
	}
	return v
}

// Positive validates that an integer is positive.
func (v *Validator) Positive(field string, value int) *Validator {
	if value <= 0 {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "must be positive",
		})
	}
	return v
}

// FloatInRange validates that a float is within the specified range.
func (v *Validator) FloatInRange(field string, value, min, max float64) *Validator {
	if value < min || value > max {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("must be between %f and %f", min, max),
		})
	}
	return v
}

// OneOf validates that a string is one of the allowed values.
func (v *Validator) OneOf(field, value string, allowed []string) *Validator {
	for _, a := range allowed {
		if value == a {
			return v
		}
	}
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: fmt.Sprintf("must be one of: %s", strings.Join(allowed, ", ")),
	})
	return v
}

// URL validates that a string is a valid URL.
func (v *Validator) URL(field, value string) *Validator {
	if value == "" {
		return v
	}
	if _, err := url.ParseRequestURI(value); err != nil {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "must be a valid URL",
		})
	}
	return v
}

// FileExists validates that a file exists at the given path.
func (v *Validator) FileExists(field, path string) *Validator {
	if path == "" {
		return v
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("file does not exist: %s", path),
		})
	}
	return v
}

// Custom runs a custom validation function.
func (v *Validator) Custom(field string, validate func() error) *Validator {
	if err := validate(); err != nil {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: err.Error(),
		})
	}
	return v
}

// Errors returns all validation errors.
func (v *Validator) Errors() ValidationErrors {
	return v.errors
}

// Validate returns an error if there are any validation errors, nil otherwise.
func (v *Validator) Validate() error {
	if v.errors.HasErrors() {
		return v.errors
	}
	return nil
}

// ValidateBaseConfig validates a BaseConfig.
func ValidateBaseConfig(cfg BaseConfig) error {
	v := NewValidator()

	v.Required("serviceName", cfg.ServiceName)
	v.OneOf("logging.level", cfg.Logging.Level, []string{"debug", "info", "warn", "error"})
	v.OneOf("logging.format", cfg.Logging.Format, []string{"json", "console"})

	v.InRange("server.port", cfg.Server.Port, 1, 65535)
	v.Positive("server.readTimeout", int(cfg.Server.ReadTimeout))
	v.Positive("server.writeTimeout", int(cfg.Server.WriteTimeout))

	if cfg.Telemetry.Enabled {
		v.Required("telemetry.endpoint", cfg.Telemetry.Endpoint)
		v.FloatInRange("telemetry.sampleRate", cfg.Telemetry.SampleRate, 0.0, 1.0)
	}

	if cfg.Server.TLS.Enabled {
		v.Required("tls.certFile", cfg.Server.TLS.CertFile)
		v.Required("tls.keyFile", cfg.Server.TLS.KeyFile)
		v.FileExists("tls.certFile", cfg.Server.TLS.CertFile)
		v.FileExists("tls.keyFile", cfg.Server.TLS.KeyFile)
		if cfg.Server.TLS.CAFile != "" {
			v.FileExists("tls.caFile", cfg.Server.TLS.CAFile)
		}
	}

	return v.Validate()
}

// ValidateKubernetesConfig validates a KubernetesConfig.
func ValidateKubernetesConfig(cfg KubernetesConfig) error {
	v := NewValidator()

	if !cfg.InCluster && cfg.KubeConfig != "" {
		v.FileExists("kubeConfig", cfg.KubeConfig)
	}

	return v.Validate()
}

// ValidateRegistryConfig validates a RegistryConfig.
func ValidateRegistryConfig(cfg RegistryConfig) error {
	v := NewValidator()

	v.Required("registry.url", cfg.URL)
	v.URL("registry.url", cfg.URL)

	return v.Validate()
}
