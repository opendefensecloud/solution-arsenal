// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracerConfig_Validation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		cfg     TracerConfig
		wantErr bool
	}{
		{
			name:    "empty service name",
			cfg:     TracerConfig{},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg: TracerConfig{
				ServiceName:    "test-service",
				ServiceVersion: "v1.0.0",
				Environment:    "test",
				Insecure:       true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp, err := InitTracer(ctx, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitTracer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tp != nil {
				_ = tp.Shutdown(ctx)
			}
		})
	}
}

func TestTracerSampling(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		samplingRatio float64
	}{
		{"never sample", 0},
		{"always sample", 1.0},
		{"ratio sample", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := TracerConfig{
				ServiceName:   "test-service",
				SamplingRatio: tt.samplingRatio,
				Insecure:      true,
			}
			tp, err := InitTracer(ctx, cfg)
			if err != nil {
				t.Fatalf("InitTracer() error = %v", err)
			}
			defer func() { _ = tp.Shutdown(ctx) }()
		})
	}
}

func TestMeterConfig_Validation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		cfg     MeterConfig
		wantErr bool
	}{
		{
			name:    "empty service name",
			cfg:     MeterConfig{},
			wantErr: true,
		},
		{
			name: "valid config",
			cfg: MeterConfig{
				ServiceName:    "test-service",
				ServiceVersion: "v1.0.0",
				Environment:    "test",
				Insecure:       true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp, err := InitMeter(ctx, tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitMeter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if mp != nil {
				_ = mp.Shutdown(ctx)
			}
		})
	}
}

func TestNewStandardMetrics(t *testing.T) {
	ctx := context.Background()

	// Initialize meter provider first
	mp, err := InitMeter(ctx, MeterConfig{
		ServiceName: "test-service",
		Insecure:    true,
	})
	if err != nil {
		t.Fatalf("InitMeter() error = %v", err)
	}
	defer func() { _ = mp.Shutdown(ctx) }()

	metrics, err := NewStandardMetrics("test-service")
	if err != nil {
		t.Fatalf("NewStandardMetrics() error = %v", err)
	}

	if metrics.RequestCounter == nil {
		t.Error("RequestCounter is nil")
	}
	if metrics.RequestDuration == nil {
		t.Error("RequestDuration is nil")
	}
	if metrics.ActiveRequests == nil {
		t.Error("ActiveRequests is nil")
	}
}

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		cfg    LogConfig
		level  string
		format string
	}{
		{
			name: "json format info level",
			cfg: LogConfig{
				Level:       "info",
				Format:      "json",
				ServiceName: "test-service",
			},
		},
		{
			name: "text format debug level",
			cfg: LogConfig{
				Level:       "debug",
				Format:      "text",
				ServiceName: "test-service",
			},
		},
		{
			name: "warn level",
			cfg: LogConfig{
				Level:       "warn",
				ServiceName: "test-service",
			},
		},
		{
			name: "error level",
			cfg: LogConfig{
				Level:       "error",
				ServiceName: "test-service",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.cfg.Output = &buf
			logger := NewLogger(tt.cfg)
			if logger == nil {
				t.Fatal("NewLogger() returned nil")
			}

			logger.Info("test message", "key", "value")
		})
	}
}

func TestNewLoggerSimple(t *testing.T) {
	logger := NewLoggerSimple("test-service")
	if logger == nil {
		t.Fatal("NewLoggerSimple() returned nil")
	}
}

func TestLoggerWithTraceContext(t *testing.T) {
	// Set up in-memory exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Create a span
	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	// Verify trace ID extraction
	traceID := TraceIDFromContext(ctx)
	if traceID == "" {
		t.Error("TraceIDFromContext() returned empty string")
	}

	spanID := SpanIDFromContext(ctx)
	if spanID == "" {
		t.Error("SpanIDFromContext() returned empty string")
	}

	// Test logging with trace context
	var buf bytes.Buffer
	logger := NewLogger(LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test-service",
		Output:      &buf,
	})

	logger.InfoContext(ctx, "test message")

	// Parse JSON output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if logEntry["trace_id"] != traceID {
		t.Errorf("trace_id = %v, want %v", logEntry["trace_id"], traceID)
	}
	if logEntry["span_id"] != spanID {
		t.Errorf("span_id = %v, want %v", logEntry["span_id"], spanID)
	}
}

func TestLoggerContext(t *testing.T) {
	logger := NewLoggerSimple("test-service")

	// Store logger in context
	ctx := ContextWithLogger(context.Background(), logger)

	// Retrieve logger from context
	retrieved := LoggerFromContext(ctx)
	if retrieved != logger {
		t.Error("LoggerFromContext() returned different logger")
	}

	// Test with empty context
	defaultLogger := LoggerFromContext(context.Background())
	if defaultLogger == nil {
		t.Error("LoggerFromContext() returned nil for empty context")
	}
}

func TestTraceIDFromContext_NoSpan(t *testing.T) {
	ctx := context.Background()
	traceID := TraceIDFromContext(ctx)
	if traceID != "" {
		t.Errorf("TraceIDFromContext() = %v, want empty string", traceID)
	}
}

func TestSpanIDFromContext_NoSpan(t *testing.T) {
	ctx := context.Background()
	spanID := SpanIDFromContext(ctx)
	if spanID != "" {
		t.Errorf("SpanIDFromContext() = %v, want empty string", spanID)
	}
}

func TestHTTPMiddleware(t *testing.T) {
	// Set up in-memory exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Create test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap with middleware
	middleware := HTTPMiddleware(HTTPMiddlewareConfig{
		ServiceName: "test-service",
		SkipPaths:   []string{"/healthz"},
	})
	wrapped := middleware(handler)

	// Test normal request
	t.Run("normal request creates span", func(t *testing.T) {
		exporter.Reset()
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		spans := exporter.GetSpans()
		if len(spans) != 1 {
			t.Errorf("got %d spans, want 1", len(spans))
		}
	})

	// Test skipped path
	t.Run("skipped path no span", func(t *testing.T) {
		exporter.Reset()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		spans := exporter.GetSpans()
		if len(spans) != 0 {
			t.Errorf("got %d spans, want 0 for skipped path", len(spans))
		}
	})
}

func TestHTTPMiddleware_ErrorStatus(t *testing.T) {
	// Set up in-memory exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	// Create error handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	middleware := HTTPMiddleware(HTTPMiddlewareConfig{ServiceName: "test-service"})
	wrapped := middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/error", nil)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// Test default status
	if rw.statusCode != http.StatusOK {
		t.Errorf("default statusCode = %d, want %d", rw.statusCode, http.StatusOK)
	}

	// Test WriteHeader
	rw.WriteHeader(http.StatusNotFound)
	if rw.statusCode != http.StatusNotFound {
		t.Errorf("statusCode = %d, want %d", rw.statusCode, http.StatusNotFound)
	}
}

func TestTracer(t *testing.T) {
	tracer := Tracer("test")
	if tracer == nil {
		t.Error("Tracer() returned nil")
	}
}

func TestMeter(t *testing.T) {
	meter := Meter("test")
	if meter == nil {
		t.Error("Meter() returned nil")
	}
}

func TestSpanFromContext(t *testing.T) {
	// Without span
	span := SpanFromContext(context.Background())
	if span == nil {
		t.Error("SpanFromContext() returned nil")
	}
}

func TestMetadataCarrier(t *testing.T) {
	mc := metadataCarrier{
		"key1": []string{"value1"},
		"key2": []string{"value2a", "value2b"},
	}

	// Test Get
	if got := mc.Get("key1"); got != "value1" {
		t.Errorf("Get(key1) = %q, want %q", got, "value1")
	}
	if got := mc.Get("key2"); got != "value2a" {
		t.Errorf("Get(key2) = %q, want %q", got, "value2a")
	}
	if got := mc.Get("missing"); got != "" {
		t.Errorf("Get(missing) = %q, want empty", got)
	}

	// Test Set
	mc.Set("key3", "value3")
	if got := mc.Get("key3"); got != "value3" {
		t.Errorf("after Set, Get(key3) = %q, want %q", got, "value3")
	}

	// Test Keys
	keys := mc.Keys()
	if len(keys) != 3 {
		t.Errorf("Keys() returned %d keys, want 3", len(keys))
	}
}

func TestTraceContextHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test",
		Output:      &buf,
	})

	// Test WithAttrs via logger.With
	childLogger := logger.With("extra", "attr")
	childLogger.Info("test")

	if buf.Len() == 0 {
		t.Error("expected log output")
	}
}

func TestTraceContextHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: "test",
		Output:      &buf,
	})

	// Test WithGroup
	groupLogger := logger.WithGroup("mygroup")
	groupLogger.Info("test", "key", "value")

	if buf.Len() == 0 {
		t.Error("expected log output")
	}
}
