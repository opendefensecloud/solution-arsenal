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
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()

	assert.Equal(t, 5*time.Minute, config.ScanInterval.Duration())
	assert.Equal(t, 5, config.Concurrency)
	assert.Empty(t, config.Registries)
	assert.True(t, config.SolarIndex.InCluster)
	assert.True(t, config.Metrics.Enabled)
	assert.Equal(t, ":8080", config.Metrics.Address)
	assert.Equal(t, "/metrics", config.Metrics.Path)
	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
}

func TestParseConfig(t *testing.T) {
	t.Parallel()

	yamlConfig := `
scanInterval: 10m
concurrency: 10
registries:
  - name: github
    url: https://ghcr.io
    repositories:
      - example/repo1
      - example/repo2
    labels:
      source: github
    namespace: default
    auth:
      type: basic
      username: user
      password: pass
  - name: docker
    url: https://registry.example.com
    auth:
      type: bearer
      token: mytoken
solarIndex:
  url: https://solar-index.example.com
  inCluster: false
metrics:
  enabled: true
  address: ":9090"
  path: /custom-metrics
logging:
  level: debug
  format: text
`

	config, err := ParseConfig([]byte(yamlConfig))
	require.NoError(t, err)

	assert.Equal(t, 10*time.Minute, config.ScanInterval.Duration())
	assert.Equal(t, 10, config.Concurrency)

	require.Len(t, config.Registries, 2)

	// Check first registry
	assert.Equal(t, "github", config.Registries[0].Name)
	assert.Equal(t, "https://ghcr.io", config.Registries[0].URL)
	assert.Equal(t, []string{"example/repo1", "example/repo2"}, config.Registries[0].Repositories)
	assert.Equal(t, "github", config.Registries[0].Labels["source"])
	assert.Equal(t, "default", config.Registries[0].Namespace)
	require.NotNil(t, config.Registries[0].Auth)
	assert.Equal(t, "basic", config.Registries[0].Auth.Type)
	assert.Equal(t, "user", config.Registries[0].Auth.Username)
	assert.Equal(t, "pass", config.Registries[0].Auth.Password)

	// Check second registry
	assert.Equal(t, "docker", config.Registries[1].Name)
	assert.Equal(t, "bearer", config.Registries[1].Auth.Type)
	assert.Equal(t, "mytoken", config.Registries[1].Auth.Token)

	// Check solar-index config
	assert.Equal(t, "https://solar-index.example.com", config.SolarIndex.URL)
	assert.False(t, config.SolarIndex.InCluster)

	// Check metrics
	assert.True(t, config.Metrics.Enabled)
	assert.Equal(t, ":9090", config.Metrics.Address)
	assert.Equal(t, "/custom-metrics", config.Metrics.Path)

	// Check logging
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
scanInterval: 1m
concurrency: 2
registries:
  - name: test
    url: https://test.example.com
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, time.Minute, config.ScanInterval.Duration())
	assert.Equal(t, 2, config.Concurrency)
	require.Len(t, config.Registries, 1)
	assert.Equal(t, "test", config.Registries[0].Name)
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	t.Parallel()

	_, err := LoadConfig("/nonexistent/path/config.yaml")
	assert.Error(t, err)
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				ScanInterval: Duration(time.Minute),
				Concurrency:  5,
				Registries: []RegistryConfig{
					{Name: "test", URL: "https://example.com"},
				},
			},
			wantErr: false,
		},
		{
			name: "scan interval too short",
			config: &Config{
				ScanInterval: Duration(100 * time.Millisecond),
				Concurrency:  5,
			},
			wantErr: true,
			errMsg:  "scanInterval must be at least 1s",
		},
		{
			name: "concurrency too low",
			config: &Config{
				ScanInterval: Duration(time.Minute),
				Concurrency:  0,
			},
			wantErr: true,
			errMsg:  "concurrency must be at least 1",
		},
		{
			name: "registry without name",
			config: &Config{
				ScanInterval: Duration(time.Minute),
				Concurrency:  5,
				Registries: []RegistryConfig{
					{URL: "https://example.com"},
				},
			},
			wantErr: true,
			errMsg:  "registries[0].name is required",
		},
		{
			name: "registry without url",
			config: &Config{
				ScanInterval: Duration(time.Minute),
				Concurrency:  5,
				Registries: []RegistryConfig{
					{Name: "test"},
				},
			},
			wantErr: true,
			errMsg:  "registries[0].url is required",
		},
		{
			name: "invalid auth config",
			config: &Config{
				ScanInterval: Duration(time.Minute),
				Concurrency:  5,
				Registries: []RegistryConfig{
					{
						Name: "test",
						URL:  "https://example.com",
						Auth: &AuthConfig{
							Type: "basic",
							// Missing username
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "username is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAuthConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		auth    *AuthConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid basic auth",
			auth: &AuthConfig{
				Type:     "basic",
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name: "basic auth with secret ref",
			auth: &AuthConfig{
				Type:     "basic",
				Username: "user",
				SecretRef: &SecretRef{
					Name: "my-secret",
				},
			},
			wantErr: false,
		},
		{
			name: "basic auth without password",
			auth: &AuthConfig{
				Type:     "basic",
				Username: "user",
			},
			wantErr: true,
			errMsg:  "password or secretRef is required",
		},
		{
			name: "basic auth without username",
			auth: &AuthConfig{
				Type:     "basic",
				Password: "pass",
			},
			wantErr: true,
			errMsg:  "username is required",
		},
		{
			name: "valid bearer auth",
			auth: &AuthConfig{
				Type:  "bearer",
				Token: "mytoken",
			},
			wantErr: false,
		},
		{
			name: "bearer auth without token",
			auth: &AuthConfig{
				Type: "bearer",
			},
			wantErr: true,
			errMsg:  "token or secretRef is required",
		},
		{
			name: "docker-config auth",
			auth: &AuthConfig{
				Type: "docker-config",
			},
			wantErr: false,
		},
		{
			name:    "empty auth type defaults to docker-config",
			auth:    &AuthConfig{},
			wantErr: false,
		},
		{
			name: "unknown auth type",
			auth: &AuthConfig{
				Type: "unknown",
			},
			wantErr: true,
			errMsg:  "unknown auth type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.auth.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_ToControllerConfig(t *testing.T) {
	t.Parallel()

	enabled := true
	disabled := false

	config := &Config{
		ScanInterval: Duration(10 * time.Minute),
		Concurrency:  8,
		Registries: []RegistryConfig{
			{
				Name:         "enabled",
				URL:          "https://enabled.example.com",
				Repositories: []string{"repo1"},
				Labels:       map[string]string{"env": "prod"},
				Namespace:    "production",
				Enabled:      &enabled,
			},
			{
				Name:    "disabled",
				URL:     "https://disabled.example.com",
				Enabled: &disabled,
			},
			{
				Name: "default-enabled",
				URL:  "https://default.example.com",
				// Enabled is nil, should default to enabled
			},
		},
	}

	registries, opts := config.ToControllerConfig()

	// Should have 2 registries (disabled one excluded)
	require.Len(t, registries, 2)

	assert.Equal(t, "enabled", registries[0].Name)
	assert.Equal(t, "https://enabled.example.com", registries[0].URL)
	assert.Equal(t, []string{"repo1"}, registries[0].Repositories)
	assert.Equal(t, "prod", registries[0].Labels["env"])
	assert.Equal(t, "production", registries[0].Namespace)

	assert.Equal(t, "default-enabled", registries[1].Name)

	// Should have 2 options
	assert.Len(t, opts, 2)
}

func TestAuthConfig_ToAuthenticator(t *testing.T) {
	t.Parallel()

	// Basic auth
	basicAuth := &AuthConfig{
		Type:     "basic",
		Username: "user",
		Password: "pass",
	}
	assert.NotNil(t, basicAuth.ToAuthenticator())

	// Bearer auth
	bearerAuth := &AuthConfig{
		Type:  "bearer",
		Token: "token",
	}
	assert.NotNil(t, bearerAuth.ToAuthenticator())

	// Nil auth
	var nilAuth *AuthConfig
	assert.Nil(t, nilAuth.ToAuthenticator())
}

func TestRegistryConfig_IsEnabled(t *testing.T) {
	t.Parallel()

	enabled := true
	disabled := false

	tests := []struct {
		name     string
		registry RegistryConfig
		want     bool
	}{
		{
			name:     "nil enabled (default)",
			registry: RegistryConfig{},
			want:     true,
		},
		{
			name:     "explicitly enabled",
			registry: RegistryConfig{Enabled: &enabled},
			want:     true,
		},
		{
			name:     "explicitly disabled",
			registry: RegistryConfig{Enabled: &disabled},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.registry.IsEnabled())
		})
	}
}

func TestDuration_MarshalYAML(t *testing.T) {
	t.Parallel()

	d := Duration(5 * time.Minute)
	result, err := d.MarshalYAML()
	require.NoError(t, err)
	assert.Equal(t, "5m0s", result)
}

func TestDuration_UnmarshalYAML_Invalid(t *testing.T) {
	t.Parallel()

	config := `
scanInterval: invalid
`
	_, err := ParseConfig([]byte(config))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid duration")
}
