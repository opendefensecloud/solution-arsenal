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

package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// MeterConfig holds configuration for the meter.
type MeterConfig struct {
	// ServiceName is the name of the service for metrics.
	ServiceName string
	// ServiceVersion is the version of the service.
	ServiceVersion string
	// Endpoint is the OTLP collector endpoint (e.g., "localhost:4317").
	Endpoint string
	// Insecure disables TLS for the connection.
	Insecure bool
	// ExportInterval is the interval between metric exports.
	ExportInterval time.Duration
}

// InitMeter initializes an OpenTelemetry MeterProvider and sets it as the global provider.
// It returns a shutdown function that should be called on application termination.
func InitMeter(ctx context.Context, cfg MeterConfig) (shutdown func(context.Context) error, err error) {
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}

	// Set default export interval.
	if cfg.ExportInterval <= 0 {
		cfg.ExportInterval = 30 * time.Second
	}

	// Create resource with service information.
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Build exporter options.
	var exporterOpts []otlpmetricgrpc.Option
	if cfg.Endpoint != "" {
		exporterOpts = append(exporterOpts, otlpmetricgrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		exporterOpts = append(exporterOpts, otlpmetricgrpc.WithInsecure())
	}

	// Create OTLP metric exporter.
	exporter, err := otlpmetricgrpc.New(ctx, exporterOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	// Create MeterProvider with periodic reader.
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(cfg.ExportInterval),
			),
		),
	)

	// Set global MeterProvider.
	otel.SetMeterProvider(mp)

	return mp.Shutdown, nil
}

// Meter returns a meter for the given component name.
func Meter(componentName string) metric.Meter {
	return otel.Meter(componentName)
}

// CommonMetrics holds commonly used metrics for a service.
type CommonMetrics struct {
	// RequestsTotal counts total requests.
	RequestsTotal metric.Int64Counter
	// RequestDuration measures request latency.
	RequestDuration metric.Float64Histogram
	// ErrorsTotal counts total errors.
	ErrorsTotal metric.Int64Counter
	// ActiveRequests tracks currently active requests.
	ActiveRequests metric.Int64UpDownCounter
}

// NewCommonMetrics creates a new CommonMetrics instance for the given meter.
func NewCommonMetrics(m metric.Meter, prefix string) (*CommonMetrics, error) {
	requestsTotal, err := m.Int64Counter(
		prefix+"_requests_total",
		metric.WithDescription("Total number of requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create requests_total counter: %w", err)
	}

	requestDuration, err := m.Float64Histogram(
		prefix+"_request_duration_seconds",
		metric.WithDescription("Request duration in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request_duration histogram: %w", err)
	}

	errorsTotal, err := m.Int64Counter(
		prefix+"_errors_total",
		metric.WithDescription("Total number of errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create errors_total counter: %w", err)
	}

	activeRequests, err := m.Int64UpDownCounter(
		prefix+"_active_requests",
		metric.WithDescription("Number of currently active requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create active_requests counter: %w", err)
	}

	return &CommonMetrics{
		RequestsTotal:   requestsTotal,
		RequestDuration: requestDuration,
		ErrorsTotal:     errorsTotal,
		ActiveRequests:  activeRequests,
	}, nil
}
