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

// Package observability provides OpenTelemetry-based tracing, metrics, and
// logging utilities for Solution Arsenal components.
package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// TracerConfig holds configuration for the tracer.
type TracerConfig struct {
	// ServiceName is the name of the service for tracing.
	ServiceName string
	// ServiceVersion is the version of the service.
	ServiceVersion string
	// Endpoint is the OTLP collector endpoint (e.g., "localhost:4317").
	Endpoint string
	// Insecure disables TLS for the connection.
	Insecure bool
	// SampleRate is the trace sampling rate (0.0 to 1.0).
	SampleRate float64
}

// InitTracer initializes an OpenTelemetry TracerProvider and sets it as the global provider.
// It returns a shutdown function that should be called on application termination.
func InitTracer(ctx context.Context, cfg TracerConfig) (shutdown func(context.Context) error, err error) {
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("service name is required")
	}

	// Set default sample rate.
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 1.0
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
	var exporterOpts []otlptracegrpc.Option
	if cfg.Endpoint != "" {
		exporterOpts = append(exporterOpts, otlptracegrpc.WithEndpoint(cfg.Endpoint))
	}
	if cfg.Insecure {
		exporterOpts = append(exporterOpts, otlptracegrpc.WithInsecure())
	}

	// Create OTLP exporter.
	exporter, err := otlptracegrpc.New(ctx, exporterOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create TracerProvider with the exporter.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
	)

	// Set global TracerProvider and propagator.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// Tracer returns a tracer for the given component name.
func Tracer(componentName string) trace.Tracer {
	return otel.Tracer(componentName)
}

// SpanFromContext returns the current span from the context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// StartSpan starts a new span with the given name and returns the updated context and span.
func StartSpan(ctx context.Context, tracer trace.Tracer, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tracer.Start(ctx, spanName, opts...)
}

// RecordError records an error on the current span if one exists.
func RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.RecordError(err)
	}
}
