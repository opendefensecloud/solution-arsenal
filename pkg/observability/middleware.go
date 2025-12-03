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
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddlewareConfig holds configuration for the HTTP middleware.
type HTTPMiddlewareConfig struct {
	// Tracer is the tracer to use for creating spans.
	Tracer trace.Tracer
	// Meter is the meter to use for recording metrics.
	Meter metric.Meter
	// Logger is the logger to use for request logging.
	Logger logr.Logger
	// ServiceName is the name of the service.
	ServiceName string
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// HTTPMiddleware returns an HTTP middleware that adds tracing, metrics, and logging.
func HTTPMiddleware(cfg HTTPMiddlewareConfig) func(http.Handler) http.Handler {
	// Initialize metrics.
	requestCounter, _ := cfg.Meter.Int64Counter(
		"http_server_requests_total",
		metric.WithDescription("Total HTTP requests"),
		metric.WithUnit("{request}"),
	)

	requestDuration, _ := cfg.Meter.Float64Histogram(
		"http_server_request_duration_seconds",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
	)

	activeRequests, _ := cfg.Meter.Int64UpDownCounter(
		"http_server_active_requests",
		metric.WithDescription("Active HTTP requests"),
		metric.WithUnit("{request}"),
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Extract trace context from incoming request.
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Start a new span.
			spanName := r.Method + " " + r.URL.Path
			ctx, span := cfg.Tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					semconv.URLPath(r.URL.Path),
					semconv.URLScheme(r.URL.Scheme),
					semconv.ServerAddress(r.Host),
					semconv.UserAgentOriginal(r.UserAgent()),
				),
			)
			defer span.End()

			// Add logger with trace context to request context.
			logger := LoggerWithTraceContext(ctx, cfg.Logger)
			ctx = ContextWithLogger(ctx, logger)

			// Track active requests.
			activeRequests.Add(ctx, 1)
			defer activeRequests.Add(ctx, -1)

			// Wrap response writer to capture status code.
			wrapped := newResponseWriter(w)

			// Call the next handler.
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Calculate duration.
			duration := time.Since(start).Seconds()

			// Record metrics.
			attrs := []attribute.KeyValue{
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.HTTPResponseStatusCode(wrapped.statusCode),
				attribute.String("path", r.URL.Path),
			}

			requestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
			requestDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

			// Set span attributes for response.
			span.SetAttributes(
				semconv.HTTPResponseStatusCode(wrapped.statusCode),
				attribute.Int64("http.response.body.size", wrapped.written),
			)

			// Mark span as error if status code indicates error.
			if wrapped.statusCode >= 400 {
				span.SetAttributes(attribute.Bool("error", true))
			}

			// Log the request.
			logger.V(1).Info("HTTP request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.statusCode,
				"duration_ms", duration*1000,
				"bytes", wrapped.written,
			)
		})
	}
}

// RecoveryMiddleware returns an HTTP middleware that recovers from panics.
func RecoveryMiddleware(logger logr.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					// Get logger with trace context if available.
					log := LoggerFromContextOrDefault(r.Context(), logger)
					log.Error(nil, "Panic recovered",
						"panic", rec,
						"method", r.Method,
						"path", r.URL.Path,
					)

					// Record error on span.
					span := trace.SpanFromContext(r.Context())
					span.SetAttributes(attribute.Bool("error", true))
					span.SetAttributes(attribute.String("panic", "true"))

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
