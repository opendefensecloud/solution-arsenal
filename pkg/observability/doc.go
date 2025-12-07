// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

// Package observability provides unified tracing, metrics, and logging
// capabilities for Solution Arsenal components.
//
// The package wraps OpenTelemetry SDK to provide:
//   - Distributed tracing with OTLP export
//   - Metrics collection with OTLP export
//   - Structured logging with trace context injection
//   - HTTP middleware for automatic span creation
//   - gRPC interceptors for tracing
//
// # Usage
//
// Initialize tracing at application startup:
//
//	tp, err := observability.InitTracer(ctx, observability.TracerConfig{
//	    ServiceName:    "solar-apiserver",
//	    ServiceVersion: "v0.1.0",
//	    Environment:    "production",
//	    Endpoint:       "otel-collector:4317",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer tp.Shutdown(ctx)
//
// Initialize metrics:
//
//	mp, err := observability.InitMeter(ctx, observability.MeterConfig{
//	    ServiceName:    "solar-apiserver",
//	    ServiceVersion: "v0.1.0",
//	    Endpoint:       "otel-collector:4317",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer mp.Shutdown(ctx)
//
// Create a logger with trace context:
//
//	logger := observability.NewLogger("solar-apiserver")
//	logger.InfoContext(ctx, "request processed", "status", 200)
//
// Use HTTP middleware:
//
//	handler := observability.HTTPMiddleware(myHandler)
package observability
