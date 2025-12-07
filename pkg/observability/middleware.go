// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// HTTPMiddlewareConfig configures the HTTP middleware.
type HTTPMiddlewareConfig struct {
	// ServiceName is used for the tracer name.
	ServiceName string
	// SkipPaths are paths that should not be traced (e.g., health checks).
	SkipPaths []string
}

// HTTPMiddleware returns HTTP middleware that creates spans and records metrics.
func HTTPMiddleware(cfg HTTPMiddlewareConfig) func(http.Handler) http.Handler {
	tracer := otel.Tracer(cfg.ServiceName)
	propagator := otel.GetTextMapPropagator()

	// Create metrics
	meter := otel.Meter(cfg.ServiceName)
	requestCounter, _ := meter.Int64Counter(
		"http.server.request.total",
		metric.WithDescription("Total HTTP requests"),
	)
	requestDuration, _ := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)

	skipSet := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, path := range cfg.SkipPaths {
		skipSet[path] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip tracing for certain paths
			if _, skip := skipSet[r.URL.Path]; skip {
				next.ServeHTTP(w, r)
				return
			}

			// Extract trace context from incoming request
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Start span
			spanName := r.Method + " " + r.URL.Path
			ctx, span := tracer.Start(ctx, spanName,
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

			// Wrap response writer to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Record start time
			start := time.Now()

			// Serve request
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Record duration
			duration := time.Since(start).Seconds()

			// Add response attributes to span
			span.SetAttributes(semconv.HTTPResponseStatusCode(wrapped.statusCode))

			// Mark span as error if status >= 400
			if wrapped.statusCode >= 400 {
				span.SetAttributes(attribute.Bool("error", true))
			}

			// Record metrics
			attrs := metric.WithAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.HTTPResponseStatusCode(wrapped.statusCode),
				semconv.URLPath(r.URL.Path),
			)
			requestCounter.Add(ctx, 1, attrs)
			requestDuration.Record(ctx, duration, attrs)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// UnaryServerInterceptor returns a gRPC unary server interceptor for tracing.
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract trace context from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			ctx = propagator.Extract(ctx, metadataCarrier(md))
		}

		// Start span
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.RPCSystemGRPC,
				semconv.RPCMethod(info.FullMethod),
			),
		)
		defer span.End()

		// Call handler
		resp, err := handler(ctx, req)

		// Record error if any
		if err != nil {
			span.SetAttributes(attribute.Bool("error", true))
			st, _ := status.FromError(err)
			span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(st.Code())))
		} else {
			span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(codes.OK)))
		}

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream server interceptor for tracing.
func StreamServerInterceptor(serviceName string) grpc.StreamServerInterceptor {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		// Extract trace context from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			ctx = propagator.Extract(ctx, metadataCarrier(md))
		}

		// Start span
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.RPCSystemGRPC,
				semconv.RPCMethod(info.FullMethod),
			),
		)
		defer span.End()

		// Wrap stream with new context
		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}

		// Call handler
		err := handler(srv, wrapped)

		// Record error if any
		if err != nil {
			span.SetAttributes(attribute.Bool("error", true))
			st, _ := status.FromError(err)
			span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(st.Code())))
		} else {
			span.SetAttributes(semconv.RPCGRPCStatusCodeKey.Int(int(codes.OK)))
		}

		return err
	}
}

// wrappedServerStream wraps grpc.ServerStream to provide custom context.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// metadataCarrier adapts gRPC metadata to propagation.TextMapCarrier.
type metadataCarrier metadata.MD

func (mc metadataCarrier) Get(key string) string {
	vals := metadata.MD(mc).Get(key)
	if len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func (mc metadataCarrier) Set(key, val string) {
	metadata.MD(mc).Set(key, val)
}

func (mc metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(mc))
	for k := range mc {
		keys = append(keys, k)
	}
	return keys
}
