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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNewLogger(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     LoggerConfig
		wantErr bool
	}{
		{
			name: "production config",
			cfg: LoggerConfig{
				Level:       "info",
				Development: false,
				Encoding:    "json",
			},
			wantErr: false,
		},
		{
			name: "development config",
			cfg: LoggerConfig{
				Level:       "debug",
				Development: true,
				Encoding:    "console",
			},
			wantErr: false,
		},
		{
			name: "invalid level defaults to info",
			cfg: LoggerConfig{
				Level:       "invalid",
				Development: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger, err := NewLogger(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, logger)

			// Test that logging doesn't panic.
			logger.Info("test message", "key", "value")
		})
	}
}

func TestLoggerWithTraceContext(t *testing.T) {
	t.Parallel()

	// Create a test tracer with in-memory exporter.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")

	// Create a base logger.
	baseLogger, err := NewLogger(LoggerConfig{Level: "info", Development: true})
	require.NoError(t, err)

	t.Run("with valid trace context", func(t *testing.T) {
		ctx, span := tracer.Start(context.Background(), "test-span")
		defer span.End()

		enrichedLogger := LoggerWithTraceContext(ctx, baseLogger)
		assert.NotNil(t, enrichedLogger)

		// The logger should have trace_id and span_id.
		// We can't easily verify the values without a custom sink,
		// but we can verify it doesn't panic.
		enrichedLogger.Info("test with trace context")
	})

	t.Run("without trace context", func(t *testing.T) {
		enrichedLogger := LoggerWithTraceContext(context.Background(), baseLogger)
		assert.NotNil(t, enrichedLogger)

		// Should return the original logger without modification.
		enrichedLogger.Info("test without trace context")
	})
}

func TestContextLogger(t *testing.T) {
	t.Parallel()

	logger, err := NewLogger(LoggerConfig{Level: "info"})
	require.NoError(t, err)

	t.Run("store and retrieve logger from context", func(t *testing.T) {
		ctx := ContextWithLogger(context.Background(), logger)
		retrieved := LoggerFromContext(ctx)

		assert.NotNil(t, retrieved)
	})

	t.Run("returns discard logger when not found", func(t *testing.T) {
		retrieved := LoggerFromContext(context.Background())

		// Discard logger should not be nil and should not panic on use.
		assert.NotNil(t, retrieved)
		retrieved.Info("this should not panic")
	})

	t.Run("returns default logger when not found", func(t *testing.T) {
		defaultLogger := logger.WithName("default")
		retrieved := LoggerFromContextOrDefault(context.Background(), defaultLogger)

		assert.NotNil(t, retrieved)
	})
}

func TestTracerConfig(t *testing.T) {
	t.Parallel()

	t.Run("empty service name returns error", func(t *testing.T) {
		_, err := InitTracer(context.Background(), TracerConfig{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service name is required")
	})
}

func TestMeterConfig(t *testing.T) {
	t.Parallel()

	t.Run("empty service name returns error", func(t *testing.T) {
		_, err := InitMeter(context.Background(), MeterConfig{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service name is required")
	})
}

func TestCommonMetrics(t *testing.T) {
	t.Parallel()

	meter := noop.NewMeterProvider().Meter("test")

	metrics, err := NewCommonMetrics(meter, "test_service")
	require.NoError(t, err)

	assert.NotNil(t, metrics.RequestsTotal)
	assert.NotNil(t, metrics.RequestDuration)
	assert.NotNil(t, metrics.ErrorsTotal)
	assert.NotNil(t, metrics.ActiveRequests)

	// Test that metrics can be used without panic.
	ctx := context.Background()
	metrics.RequestsTotal.Add(ctx, 1)
	metrics.RequestDuration.Record(ctx, 0.5)
	metrics.ErrorsTotal.Add(ctx, 1)
	metrics.ActiveRequests.Add(ctx, 1)
	metrics.ActiveRequests.Add(ctx, -1)
}

func TestHTTPMiddleware(t *testing.T) {
	t.Parallel()

	// Set up test tracer.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	logger, err := NewLogger(LoggerConfig{Level: "debug", Development: true})
	require.NoError(t, err)

	cfg := HTTPMiddlewareConfig{
		Tracer:      tp.Tracer("test"),
		Meter:       noop.NewMeterProvider().Meter("test"),
		Logger:      logger,
		ServiceName: "test-service",
	}

	middleware := HTTPMiddleware(cfg)

	t.Run("successful request", func(t *testing.T) {
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "OK", rec.Body.String())
	})

	t.Run("error response", func(t *testing.T) {
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
		}))

		req := httptest.NewRequest(http.MethodPost, "/error", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestRecoveryMiddleware(t *testing.T) {
	t.Parallel()

	logger, err := NewLogger(LoggerConfig{Level: "info"})
	require.NoError(t, err)

	middleware := RecoveryMiddleware(logger)

	t.Run("recovers from panic", func(t *testing.T) {
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		}))

		req := httptest.NewRequest(http.MethodGet, "/panic", nil)
		rec := httptest.NewRecorder()

		// Should not panic.
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("passes through normal requests", func(t *testing.T) {
		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/ok", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestTracer(t *testing.T) {
	t.Parallel()

	tracer := Tracer("test-component")
	assert.NotNil(t, tracer)
}

func TestMeter(t *testing.T) {
	t.Parallel()

	meter := Meter("test-component")
	assert.NotNil(t, meter)
}

func TestLoggerWithName(t *testing.T) {
	t.Parallel()

	logger, err := NewLogger(LoggerConfig{Level: "info"})
	require.NoError(t, err)

	named := LoggerWithName(logger, "component")
	assert.NotNil(t, named)

	// Should not panic.
	named.Info("test")
}

func TestLoggerWithValues(t *testing.T) {
	t.Parallel()

	logger, err := NewLogger(LoggerConfig{Level: "info"})
	require.NoError(t, err)

	enriched := LoggerWithValues(logger, "key1", "value1", "key2", 42)
	assert.NotNil(t, enriched)

	// Should not panic.
	enriched.Info("test")
}

func TestSpanFromContext(t *testing.T) {
	t.Parallel()

	// Create a test tracer.
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")

	t.Run("returns span from context", func(t *testing.T) {
		ctx, span := tracer.Start(context.Background(), "test")
		defer span.End()

		retrieved := SpanFromContext(ctx)
		assert.NotNil(t, retrieved)
		assert.True(t, retrieved.SpanContext().IsValid())
	})

	t.Run("returns noop span when not found", func(t *testing.T) {
		retrieved := SpanFromContext(context.Background())
		assert.NotNil(t, retrieved)
		assert.False(t, retrieved.SpanContext().IsValid())
	})
}

func TestRecordError(t *testing.T) {
	t.Parallel()

	// Create a test tracer with in-memory exporter.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test")

	// Should not panic even with nil error.
	RecordError(ctx, nil)

	// Record actual error.
	RecordError(ctx, assert.AnError)

	span.End()
}

func TestStartSpan(t *testing.T) {
	t.Parallel()

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")

	ctx, span := StartSpan(context.Background(), tracer, "test-span")

	assert.NotNil(t, ctx)
	assert.NotNil(t, span)
	assert.True(t, span.SpanContext().IsValid())

	span.End()
}

// Benchmark tests for critical paths.

func BenchmarkLoggerWithTraceContext(b *testing.B) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("bench")
	ctx, span := tracer.Start(context.Background(), "bench-span")
	defer span.End()

	logger := logr.Discard()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = LoggerWithTraceContext(ctx, logger)
	}
}

func BenchmarkHTTPMiddleware(b *testing.B) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()

	cfg := HTTPMiddlewareConfig{
		Tracer:      tp.Tracer("bench"),
		Meter:       noop.NewMeterProvider().Meter("bench"),
		Logger:      logr.Discard(),
		ServiceName: "bench-service",
	}

	middleware := HTTPMiddleware(cfg)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/bench", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
