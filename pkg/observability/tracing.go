// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials/insecure"
)

// TracerConfig holds configuration for initializing the tracer provider.
type TracerConfig struct {
	// ServiceName is the name of the service being traced.
	ServiceName string
	// ServiceVersion is the version of the service.
	ServiceVersion string
	// Environment is the deployment environment (e.g., "production", "staging").
	Environment string
	// Endpoint is the OTLP collector endpoint (e.g., "otel-collector:4317").
	Endpoint string
	// Insecure disables TLS for the OTLP connection.
	Insecure bool
	// SamplingRatio is the fraction of traces to sample (0.0 to 1.0).
	// A value of 1.0 means all traces are sampled.
	SamplingRatio float64
}

// InitTracer initializes an OpenTelemetry TracerProvider with OTLP export.
// The returned TracerProvider should be shut down when the application exits
// to ensure all spans are flushed.
func InitTracer(ctx context.Context, cfg TracerConfig) (*sdktrace.TracerProvider, error) {
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
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
	opts := []otlptracegrpc.Option{}
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()))
	}

	// Create OTLP exporter
	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Configure sampler
	var sampler sdktrace.Sampler
	if cfg.SamplingRatio <= 0 {
		sampler = sdktrace.NeverSample()
	} else if cfg.SamplingRatio >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SamplingRatio)
	}

	// Create TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set as global TracerProvider
	otel.SetTracerProvider(tp)

	// Set up W3C Trace Context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

// Tracer returns a named tracer from the global TracerProvider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// SpanFromContext returns the current Span from a context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// TraceIDFromContext extracts the trace ID from a context as a string.
// Returns an empty string if no trace is present.
func TraceIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().HasTraceID() {
		return ""
	}
	return span.SpanContext().TraceID().String()
}

// SpanIDFromContext extracts the span ID from a context as a string.
// Returns an empty string if no span is present.
func SpanIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().HasSpanID() {
		return ""
	}
	return span.SpanContext().SpanID().String()
}
