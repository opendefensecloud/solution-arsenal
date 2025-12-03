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

// Package config provides configuration loading and validation for Solution Arsenal components.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// BaseConfig contains common configuration fields shared across all components.
type BaseConfig struct {
	// ServiceName is the name of the service for observability.
	ServiceName string `json:"serviceName" yaml:"serviceName"`
	// ServiceVersion is the version of the service.
	ServiceVersion string `json:"serviceVersion" yaml:"serviceVersion"`

	// Logging configuration.
	Logging LoggingConfig `json:"logging" yaml:"logging"`

	// Telemetry configuration.
	Telemetry TelemetryConfig `json:"telemetry" yaml:"telemetry"`

	// Server configuration.
	Server ServerConfig `json:"server" yaml:"server"`
}

// LoggingConfig contains logging configuration.
type LoggingConfig struct {
	// Level is the log level (debug, info, warn, error).
	Level string `json:"level" yaml:"level"`
	// Format is the log format (json, console).
	Format string `json:"format" yaml:"format"`
	// Development enables development mode.
	Development bool `json:"development" yaml:"development"`
}

// TelemetryConfig contains OpenTelemetry configuration.
type TelemetryConfig struct {
	// Enabled enables telemetry export.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Endpoint is the OTLP collector endpoint.
	Endpoint string `json:"endpoint" yaml:"endpoint"`
	// Insecure disables TLS for the telemetry connection.
	Insecure bool `json:"insecure" yaml:"insecure"`
	// SampleRate is the trace sampling rate (0.0 to 1.0).
	SampleRate float64 `json:"sampleRate" yaml:"sampleRate"`
	// ExportInterval is the metrics export interval.
	ExportInterval time.Duration `json:"exportInterval" yaml:"exportInterval"`
}

// ServerConfig contains HTTP/gRPC server configuration.
type ServerConfig struct {
	// Host is the server bind address.
	Host string `json:"host" yaml:"host"`
	// Port is the server port.
	Port int `json:"port" yaml:"port"`
	// TLS configuration.
	TLS TLSConfig `json:"tls" yaml:"tls"`
	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration `json:"readTimeout" yaml:"readTimeout"`
	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration `json:"writeTimeout" yaml:"writeTimeout"`
	// IdleTimeout is the maximum amount of time to wait for the next request.
	IdleTimeout time.Duration `json:"idleTimeout" yaml:"idleTimeout"`
	// ShutdownTimeout is the maximum duration to wait for active connections to close.
	ShutdownTimeout time.Duration `json:"shutdownTimeout" yaml:"shutdownTimeout"`
}

// TLSConfig contains TLS configuration.
type TLSConfig struct {
	// Enabled enables TLS.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// CertFile is the path to the TLS certificate file.
	CertFile string `json:"certFile" yaml:"certFile"`
	// KeyFile is the path to the TLS key file.
	KeyFile string `json:"keyFile" yaml:"keyFile"`
	// CAFile is the path to the CA certificate file for client verification.
	CAFile string `json:"caFile" yaml:"caFile"`
}

// KubernetesConfig contains Kubernetes client configuration.
type KubernetesConfig struct {
	// InCluster enables in-cluster configuration.
	InCluster bool `json:"inCluster" yaml:"inCluster"`
	// KubeConfig is the path to the kubeconfig file (used when InCluster is false).
	KubeConfig string `json:"kubeConfig" yaml:"kubeConfig"`
	// Namespace is the namespace to operate in.
	Namespace string `json:"namespace" yaml:"namespace"`
}

// RegistryConfig contains OCI registry configuration.
type RegistryConfig struct {
	// URL is the OCI registry URL.
	URL string `json:"url" yaml:"url"`
	// Username for registry authentication.
	Username string `json:"username" yaml:"username"`
	// Password for registry authentication.
	Password string `json:"password" yaml:"password"`
	// Insecure allows insecure connections.
	Insecure bool `json:"insecure" yaml:"insecure"`
}

// DefaultBaseConfig returns a BaseConfig with sensible defaults.
func DefaultBaseConfig() BaseConfig {
	return BaseConfig{
		ServiceName:    "solar",
		ServiceVersion: "dev",
		Logging: LoggingConfig{
			Level:       "info",
			Format:      "json",
			Development: false,
		},
		Telemetry: TelemetryConfig{
			Enabled:        false,
			Endpoint:       "localhost:4317",
			Insecure:       true,
			SampleRate:     1.0,
			ExportInterval: 30 * time.Second,
		},
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 30 * time.Second,
		},
	}
}

// Address returns the server address in host:port format.
func (c ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// EnvLoader loads configuration values from environment variables.
type EnvLoader struct {
	prefix string
}

// NewEnvLoader creates a new EnvLoader with the given prefix.
// Environment variables will be looked up as PREFIX_KEY (e.g., SOLAR_LOG_LEVEL).
func NewEnvLoader(prefix string) *EnvLoader {
	return &EnvLoader{prefix: strings.ToUpper(prefix)}
}

// GetString returns the string value for the given key, or the default if not set.
func (l *EnvLoader) GetString(key, defaultValue string) string {
	envKey := l.envKey(key)
	if value := os.Getenv(envKey); value != "" {
		return value
	}
	return defaultValue
}

// GetInt returns the int value for the given key, or the default if not set or invalid.
func (l *EnvLoader) GetInt(key string, defaultValue int) int {
	envKey := l.envKey(key)
	if value := os.Getenv(envKey); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// GetBool returns the bool value for the given key, or the default if not set or invalid.
func (l *EnvLoader) GetBool(key string, defaultValue bool) bool {
	envKey := l.envKey(key)
	if value := os.Getenv(envKey); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

// GetFloat returns the float64 value for the given key, or the default if not set or invalid.
func (l *EnvLoader) GetFloat(key string, defaultValue float64) float64 {
	envKey := l.envKey(key)
	if value := os.Getenv(envKey); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

// GetDuration returns the duration value for the given key, or the default if not set or invalid.
func (l *EnvLoader) GetDuration(key string, defaultValue time.Duration) time.Duration {
	envKey := l.envKey(key)
	if value := os.Getenv(envKey); value != "" {
		if durVal, err := time.ParseDuration(value); err == nil {
			return durVal
		}
	}
	return defaultValue
}

func (l *EnvLoader) envKey(key string) string {
	key = strings.ToUpper(key)
	key = strings.ReplaceAll(key, ".", "_")
	key = strings.ReplaceAll(key, "-", "_")
	if l.prefix != "" {
		return l.prefix + "_" + key
	}
	return key
}

// LoadBaseConfigFromEnv loads BaseConfig from environment variables.
func LoadBaseConfigFromEnv(prefix string) BaseConfig {
	loader := NewEnvLoader(prefix)
	cfg := DefaultBaseConfig()

	cfg.ServiceName = loader.GetString("SERVICE_NAME", cfg.ServiceName)
	cfg.ServiceVersion = loader.GetString("SERVICE_VERSION", cfg.ServiceVersion)

	cfg.Logging.Level = loader.GetString("LOG_LEVEL", cfg.Logging.Level)
	cfg.Logging.Format = loader.GetString("LOG_FORMAT", cfg.Logging.Format)
	cfg.Logging.Development = loader.GetBool("LOG_DEVELOPMENT", cfg.Logging.Development)

	cfg.Telemetry.Enabled = loader.GetBool("TELEMETRY_ENABLED", cfg.Telemetry.Enabled)
	cfg.Telemetry.Endpoint = loader.GetString("TELEMETRY_ENDPOINT", cfg.Telemetry.Endpoint)
	cfg.Telemetry.Insecure = loader.GetBool("TELEMETRY_INSECURE", cfg.Telemetry.Insecure)
	cfg.Telemetry.SampleRate = loader.GetFloat("TELEMETRY_SAMPLE_RATE", cfg.Telemetry.SampleRate)
	cfg.Telemetry.ExportInterval = loader.GetDuration("TELEMETRY_EXPORT_INTERVAL", cfg.Telemetry.ExportInterval)

	cfg.Server.Host = loader.GetString("SERVER_HOST", cfg.Server.Host)
	cfg.Server.Port = loader.GetInt("SERVER_PORT", cfg.Server.Port)
	cfg.Server.ReadTimeout = loader.GetDuration("SERVER_READ_TIMEOUT", cfg.Server.ReadTimeout)
	cfg.Server.WriteTimeout = loader.GetDuration("SERVER_WRITE_TIMEOUT", cfg.Server.WriteTimeout)
	cfg.Server.IdleTimeout = loader.GetDuration("SERVER_IDLE_TIMEOUT", cfg.Server.IdleTimeout)
	cfg.Server.ShutdownTimeout = loader.GetDuration("SERVER_SHUTDOWN_TIMEOUT", cfg.Server.ShutdownTimeout)

	cfg.Server.TLS.Enabled = loader.GetBool("TLS_ENABLED", cfg.Server.TLS.Enabled)
	cfg.Server.TLS.CertFile = loader.GetString("TLS_CERT_FILE", cfg.Server.TLS.CertFile)
	cfg.Server.TLS.KeyFile = loader.GetString("TLS_KEY_FILE", cfg.Server.TLS.KeyFile)
	cfg.Server.TLS.CAFile = loader.GetString("TLS_CA_FILE", cfg.Server.TLS.CAFile)

	return cfg
}

// LoadKubernetesConfigFromEnv loads KubernetesConfig from environment variables.
func LoadKubernetesConfigFromEnv(prefix string) KubernetesConfig {
	loader := NewEnvLoader(prefix)
	return KubernetesConfig{
		InCluster:  loader.GetBool("KUBE_IN_CLUSTER", true),
		KubeConfig: loader.GetString("KUBECONFIG", ""),
		Namespace:  loader.GetString("KUBE_NAMESPACE", "default"),
	}
}

// LoadRegistryConfigFromEnv loads RegistryConfig from environment variables.
func LoadRegistryConfigFromEnv(prefix string) RegistryConfig {
	loader := NewEnvLoader(prefix)
	return RegistryConfig{
		URL:      loader.GetString("REGISTRY_URL", ""),
		Username: loader.GetString("REGISTRY_USERNAME", ""),
		Password: loader.GetString("REGISTRY_PASSWORD", ""),
		Insecure: loader.GetBool("REGISTRY_INSECURE", false),
	}
}
