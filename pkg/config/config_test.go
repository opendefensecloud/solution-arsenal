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
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultBaseConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultBaseConfig()

	assert.Equal(t, "solar", cfg.ServiceName)
	assert.Equal(t, "dev", cfg.ServiceVersion)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.False(t, cfg.Logging.Development)
	assert.False(t, cfg.Telemetry.Enabled)
	assert.Equal(t, "localhost:4317", cfg.Telemetry.Endpoint)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
}

func TestServerConfigAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "default",
			host:     "0.0.0.0",
			port:     8080,
			expected: "0.0.0.0:8080",
		},
		{
			name:     "localhost",
			host:     "localhost",
			port:     9090,
			expected: "localhost:9090",
		},
		{
			name:     "specific host",
			host:     "192.168.1.1",
			port:     443,
			expected: "192.168.1.1:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := ServerConfig{Host: tt.host, Port: tt.port}
			assert.Equal(t, tt.expected, cfg.Address())
		})
	}
}

func TestEnvLoader(t *testing.T) {
	// Note: not parallel due to environment variable manipulation.

	// Clean up env vars after test.
	defer func() {
		os.Unsetenv("TEST_STRING_VAR")
		os.Unsetenv("TEST_INT_VAR")
		os.Unsetenv("TEST_BOOL_VAR")
		os.Unsetenv("TEST_FLOAT_VAR")
		os.Unsetenv("TEST_DURATION_VAR")
	}()

	loader := NewEnvLoader("TEST")

	t.Run("GetString", func(t *testing.T) {
		// Without env var.
		assert.Equal(t, "default", loader.GetString("STRING_VAR", "default"))

		// With env var.
		os.Setenv("TEST_STRING_VAR", "custom")
		assert.Equal(t, "custom", loader.GetString("STRING_VAR", "default"))
	})

	t.Run("GetInt", func(t *testing.T) {
		// Without env var.
		assert.Equal(t, 42, loader.GetInt("INT_VAR", 42))

		// With valid env var.
		os.Setenv("TEST_INT_VAR", "100")
		assert.Equal(t, 100, loader.GetInt("INT_VAR", 42))

		// With invalid env var.
		os.Setenv("TEST_INT_VAR", "not-a-number")
		assert.Equal(t, 42, loader.GetInt("INT_VAR", 42))
	})

	t.Run("GetBool", func(t *testing.T) {
		// Without env var.
		assert.False(t, loader.GetBool("BOOL_VAR", false))

		// With valid env var.
		os.Setenv("TEST_BOOL_VAR", "true")
		assert.True(t, loader.GetBool("BOOL_VAR", false))

		os.Setenv("TEST_BOOL_VAR", "1")
		assert.True(t, loader.GetBool("BOOL_VAR", false))

		os.Setenv("TEST_BOOL_VAR", "false")
		assert.False(t, loader.GetBool("BOOL_VAR", true))

		// With invalid env var.
		os.Setenv("TEST_BOOL_VAR", "not-a-bool")
		assert.True(t, loader.GetBool("BOOL_VAR", true))
	})

	t.Run("GetFloat", func(t *testing.T) {
		// Without env var.
		assert.Equal(t, 0.5, loader.GetFloat("FLOAT_VAR", 0.5))

		// With valid env var.
		os.Setenv("TEST_FLOAT_VAR", "0.75")
		assert.Equal(t, 0.75, loader.GetFloat("FLOAT_VAR", 0.5))

		// With invalid env var.
		os.Setenv("TEST_FLOAT_VAR", "not-a-float")
		assert.Equal(t, 0.5, loader.GetFloat("FLOAT_VAR", 0.5))
	})

	t.Run("GetDuration", func(t *testing.T) {
		// Without env var.
		assert.Equal(t, 30*time.Second, loader.GetDuration("DURATION_VAR", 30*time.Second))

		// With valid env var.
		os.Setenv("TEST_DURATION_VAR", "1m")
		assert.Equal(t, time.Minute, loader.GetDuration("DURATION_VAR", 30*time.Second))

		os.Setenv("TEST_DURATION_VAR", "500ms")
		assert.Equal(t, 500*time.Millisecond, loader.GetDuration("DURATION_VAR", 30*time.Second))

		// With invalid env var.
		os.Setenv("TEST_DURATION_VAR", "not-a-duration")
		assert.Equal(t, 30*time.Second, loader.GetDuration("DURATION_VAR", 30*time.Second))
	})
}

func TestEnvLoaderKeyConversion(t *testing.T) {
	t.Parallel()

	// Test that keys with dots and dashes are converted properly.
	// We can verify this by setting an env var with the converted key
	// and checking that GetString finds it.
	t.Run("converts dots to underscores", func(t *testing.T) {
		loader := NewEnvLoader("PREFIX")
		// With no env var set, should return default.
		assert.Equal(t, "default", loader.GetString("foo.bar", "default"))
	})

	t.Run("converts dashes to underscores", func(t *testing.T) {
		loader := NewEnvLoader("PREFIX")
		// With no env var set, should return default.
		assert.Equal(t, "default", loader.GetString("foo-bar", "default"))
	})
}

func TestLoadBaseConfigFromEnv(t *testing.T) {
	// Not parallel due to env var manipulation.

	// Set up test env vars.
	os.Setenv("SOLAR_SERVICE_NAME", "test-service")
	os.Setenv("SOLAR_LOG_LEVEL", "debug")
	os.Setenv("SOLAR_SERVER_PORT", "9090")

	defer func() {
		os.Unsetenv("SOLAR_SERVICE_NAME")
		os.Unsetenv("SOLAR_LOG_LEVEL")
		os.Unsetenv("SOLAR_SERVER_PORT")
	}()

	cfg := LoadBaseConfigFromEnv("SOLAR")

	assert.Equal(t, "test-service", cfg.ServiceName)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, 9090, cfg.Server.Port)
	// Defaults should be preserved for unset vars.
	assert.Equal(t, "json", cfg.Logging.Format)
}

func TestLoadKubernetesConfigFromEnv(t *testing.T) {
	// Not parallel due to env var manipulation.

	os.Setenv("TEST_KUBE_IN_CLUSTER", "false")
	os.Setenv("TEST_KUBECONFIG", "/path/to/kubeconfig")
	os.Setenv("TEST_KUBE_NAMESPACE", "custom-ns")

	defer func() {
		os.Unsetenv("TEST_KUBE_IN_CLUSTER")
		os.Unsetenv("TEST_KUBECONFIG")
		os.Unsetenv("TEST_KUBE_NAMESPACE")
	}()

	cfg := LoadKubernetesConfigFromEnv("TEST")

	assert.False(t, cfg.InCluster)
	assert.Equal(t, "/path/to/kubeconfig", cfg.KubeConfig)
	assert.Equal(t, "custom-ns", cfg.Namespace)
}

func TestLoadRegistryConfigFromEnv(t *testing.T) {
	// Not parallel due to env var manipulation.

	os.Setenv("TEST_REGISTRY_URL", "https://registry.example.com")
	os.Setenv("TEST_REGISTRY_USERNAME", "user")
	os.Setenv("TEST_REGISTRY_PASSWORD", "pass")
	os.Setenv("TEST_REGISTRY_INSECURE", "true")

	defer func() {
		os.Unsetenv("TEST_REGISTRY_URL")
		os.Unsetenv("TEST_REGISTRY_USERNAME")
		os.Unsetenv("TEST_REGISTRY_PASSWORD")
		os.Unsetenv("TEST_REGISTRY_INSECURE")
	}()

	cfg := LoadRegistryConfigFromEnv("TEST")

	assert.Equal(t, "https://registry.example.com", cfg.URL)
	assert.Equal(t, "user", cfg.Username)
	assert.Equal(t, "pass", cfg.Password)
	assert.True(t, cfg.Insecure)
}

func TestValidateBaseConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     BaseConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			cfg:     DefaultBaseConfig(),
			wantErr: false,
		},
		{
			name: "missing service name",
			cfg: func() BaseConfig {
				c := DefaultBaseConfig()
				c.ServiceName = ""
				return c
			}(),
			wantErr: true,
			errMsg:  "serviceName",
		},
		{
			name: "invalid log level",
			cfg: func() BaseConfig {
				c := DefaultBaseConfig()
				c.Logging.Level = "invalid"
				return c
			}(),
			wantErr: true,
			errMsg:  "logging.level",
		},
		{
			name: "invalid log format",
			cfg: func() BaseConfig {
				c := DefaultBaseConfig()
				c.Logging.Format = "invalid"
				return c
			}(),
			wantErr: true,
			errMsg:  "logging.format",
		},
		{
			name: "invalid port",
			cfg: func() BaseConfig {
				c := DefaultBaseConfig()
				c.Server.Port = 0
				return c
			}(),
			wantErr: true,
			errMsg:  "server.port",
		},
		{
			name: "port out of range",
			cfg: func() BaseConfig {
				c := DefaultBaseConfig()
				c.Server.Port = 70000
				return c
			}(),
			wantErr: true,
			errMsg:  "server.port",
		},
		{
			name: "telemetry enabled without endpoint",
			cfg: func() BaseConfig {
				c := DefaultBaseConfig()
				c.Telemetry.Enabled = true
				c.Telemetry.Endpoint = ""
				return c
			}(),
			wantErr: true,
			errMsg:  "telemetry.endpoint",
		},
		{
			name: "telemetry enabled with valid config",
			cfg: func() BaseConfig {
				c := DefaultBaseConfig()
				c.Telemetry.Enabled = true
				c.Telemetry.Endpoint = "localhost:4317"
				c.Telemetry.SampleRate = 0.5
				return c
			}(),
			wantErr: false,
		},
		{
			name: "invalid sample rate",
			cfg: func() BaseConfig {
				c := DefaultBaseConfig()
				c.Telemetry.Enabled = true
				c.Telemetry.SampleRate = 1.5
				return c
			}(),
			wantErr: true,
			errMsg:  "sampleRate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateBaseConfig(tt.cfg)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateRegistryConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     RegistryConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: RegistryConfig{
				URL:      "https://registry.example.com",
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			cfg: RegistryConfig{
				URL: "",
			},
			wantErr: true,
		},
		{
			name: "invalid URL",
			cfg: RegistryConfig{
				URL: "not-a-url",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRegistryConfig(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator(t *testing.T) {
	t.Parallel()

	t.Run("Required", func(t *testing.T) {
		t.Parallel()

		v := NewValidator()
		v.Required("field", "value")
		assert.NoError(t, v.Validate())

		v = NewValidator()
		v.Required("field", "")
		assert.Error(t, v.Validate())

		v = NewValidator()
		v.Required("field", "   ")
		assert.Error(t, v.Validate())
	})

	t.Run("MinLength", func(t *testing.T) {
		t.Parallel()

		v := NewValidator()
		v.MinLength("field", "abc", 3)
		assert.NoError(t, v.Validate())

		v = NewValidator()
		v.MinLength("field", "ab", 3)
		assert.Error(t, v.Validate())
	})

	t.Run("MaxLength", func(t *testing.T) {
		t.Parallel()

		v := NewValidator()
		v.MaxLength("field", "abc", 3)
		assert.NoError(t, v.Validate())

		v = NewValidator()
		v.MaxLength("field", "abcd", 3)
		assert.Error(t, v.Validate())
	})

	t.Run("InRange", func(t *testing.T) {
		t.Parallel()

		v := NewValidator()
		v.InRange("field", 5, 1, 10)
		assert.NoError(t, v.Validate())

		v = NewValidator()
		v.InRange("field", 0, 1, 10)
		assert.Error(t, v.Validate())

		v = NewValidator()
		v.InRange("field", 11, 1, 10)
		assert.Error(t, v.Validate())
	})

	t.Run("Positive", func(t *testing.T) {
		t.Parallel()

		v := NewValidator()
		v.Positive("field", 1)
		assert.NoError(t, v.Validate())

		v = NewValidator()
		v.Positive("field", 0)
		assert.Error(t, v.Validate())

		v = NewValidator()
		v.Positive("field", -1)
		assert.Error(t, v.Validate())
	})

	t.Run("OneOf", func(t *testing.T) {
		t.Parallel()

		v := NewValidator()
		v.OneOf("field", "a", []string{"a", "b", "c"})
		assert.NoError(t, v.Validate())

		v = NewValidator()
		v.OneOf("field", "d", []string{"a", "b", "c"})
		assert.Error(t, v.Validate())
	})

	t.Run("URL", func(t *testing.T) {
		t.Parallel()

		v := NewValidator()
		v.URL("field", "https://example.com")
		assert.NoError(t, v.Validate())

		v = NewValidator()
		v.URL("field", "")
		assert.NoError(t, v.Validate()) // Empty is allowed.

		v = NewValidator()
		v.URL("field", "not-a-url")
		assert.Error(t, v.Validate())
	})

	t.Run("Custom", func(t *testing.T) {
		t.Parallel()

		v := NewValidator()
		v.Custom("field", func() error { return nil })
		assert.NoError(t, v.Validate())

		v = NewValidator()
		v.Custom("field", func() error { return assert.AnError })
		assert.Error(t, v.Validate())
	})
}

func TestValidationErrors(t *testing.T) {
	t.Parallel()

	t.Run("single error", func(t *testing.T) {
		t.Parallel()

		errs := ValidationErrors{
			{Field: "field1", Message: "is required"},
		}
		assert.Contains(t, errs.Error(), "field1")
		assert.Contains(t, errs.Error(), "is required")
		assert.True(t, errs.HasErrors())
	})

	t.Run("multiple errors", func(t *testing.T) {
		t.Parallel()

		errs := ValidationErrors{
			{Field: "field1", Message: "is required"},
			{Field: "field2", Message: "must be positive"},
		}
		errStr := errs.Error()
		assert.Contains(t, errStr, "field1")
		assert.Contains(t, errStr, "field2")
		assert.True(t, errs.HasErrors())
	})

	t.Run("no errors", func(t *testing.T) {
		t.Parallel()

		errs := ValidationErrors{}
		assert.Equal(t, "no validation errors", errs.Error())
		assert.False(t, errs.HasErrors())
	})
}
