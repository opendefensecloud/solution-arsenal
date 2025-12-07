// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

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
	"google.golang.org/grpc/credentials/insecure"
)

// MeterConfig holds configuration for initializing the meter provider.
type MeterConfig struct {
	// ServiceName is the name of the service being metered.
	ServiceName string
	// ServiceVersion is the version of the service.
	ServiceVersion string
	// Environment is the deployment environment.
	Environment string
	// Endpoint is the OTLP collector endpoint (e.g., "otel-collector:4317").
	Endpoint string
	// Insecure disables TLS for the OTLP connection.
	Insecure bool
	// Interval is the metric export interval. Defaults to 60 seconds.
	Interval time.Duration
}

// InitMeter initializes an OpenTelemetry MeterProvider with OTLP export.
// The returned MeterProvider should be shut down when the application exits
// to ensure all metrics are flushed.
func InitMeter(ctx context.Context, cfg MeterConfig) (*sdkmetric.MeterProvider, error) {
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}

	// Set default interval
	if cfg.Interval <= 0 {
		cfg.Interval = 60 * time.Second
	}

	// Build resource with service information
	// Note: We create a new resource without merging with Default() to avoid
	// schema URL conflicts between SDK and semconv versions.
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
		resource.WithProcessRuntimeDescription(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Configure OTLP exporter options
	opts := []otlpmetricgrpc.Option{}
	if cfg.Endpoint != "" {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithTLSCredentials(insecure.NewCredentials()))
	}

	// Create OTLP exporter
	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	// Create MeterProvider with periodic reader
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				exporter,
				sdkmetric.WithInterval(cfg.Interval),
			),
		),
	)

	// Set as global MeterProvider
	otel.SetMeterProvider(mp)

	return mp, nil
}

// Meter returns a named meter from the global MeterProvider.
func Meter(name string) metric.Meter {
	return otel.Meter(name)
}

// StandardMetrics holds common metrics used across services.
type StandardMetrics struct {
	// RequestCounter counts total requests by method and status.
	RequestCounter metric.Int64Counter
	// RequestDuration records request latency.
	RequestDuration metric.Float64Histogram
	// ActiveRequests tracks current in-flight requests.
	ActiveRequests metric.Int64UpDownCounter
}

// NewStandardMetrics creates a set of standard metrics for a service.
func NewStandardMetrics(serviceName string) (*StandardMetrics, error) {
	meter := Meter(serviceName)

	requestCounter, err := meter.Int64Counter(
		"http.server.request.total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request counter: %w", err)
	}

	requestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request latency in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	activeRequests, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create active requests counter: %w", err)
	}

	return &StandardMetrics{
		RequestCounter:  requestCounter,
		RequestDuration: requestDuration,
		ActiveRequests:  activeRequests,
	}, nil
}
