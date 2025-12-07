// Copyright 2025 BWI GmbH and Artifact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// LogConfig holds configuration for the logger.
type LogConfig struct {
	// Level is the minimum log level (debug, info, warn, error).
	Level string
	// Format is the output format (json, text).
	Format string
	// Output is the writer for log output. Defaults to os.Stdout.
	Output io.Writer
	// ServiceName is included in all log entries.
	ServiceName string
	// ServiceVersion is included in all log entries.
	ServiceVersion string
}

// NewLogger creates a new structured logger with the given configuration.
// The logger automatically injects trace and span IDs when available in context.
func NewLogger(cfg LogConfig) *slog.Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}

	// Parse log level
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	// Create handler based on format
	var handler slog.Handler
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(cfg.Output, opts)
	} else {
		handler = slog.NewJSONHandler(cfg.Output, opts)
	}

	// Wrap with trace context handler
	handler = &traceContextHandler{Handler: handler}

	// Create logger with service attributes
	logger := slog.New(handler)
	if cfg.ServiceName != "" {
		logger = logger.With(slog.String("service", cfg.ServiceName))
	}
	if cfg.ServiceVersion != "" {
		logger = logger.With(slog.String("version", cfg.ServiceVersion))
	}

	return logger
}

// NewLoggerSimple creates a logger with just a service name using defaults.
func NewLoggerSimple(serviceName string) *slog.Logger {
	return NewLogger(LogConfig{
		Level:       "info",
		Format:      "json",
		ServiceName: serviceName,
	})
}

// traceContextHandler wraps a slog.Handler to inject trace context.
type traceContextHandler struct {
	slog.Handler
}

// Handle adds trace context attributes if available.
func (h *traceContextHandler) Handle(ctx context.Context, r slog.Record) error {
	// Add trace ID if present
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		r.AddAttrs(slog.String("trace_id", traceID))
	}

	// Add span ID if present
	if spanID := SpanIDFromContext(ctx); spanID != "" {
		r.AddAttrs(slog.String("span_id", spanID))
	}

	return h.Handler.Handle(ctx, r)
}

// WithAttrs returns a new handler with additional attributes.
func (h *traceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with a group name.
func (h *traceContextHandler) WithGroup(name string) slog.Handler {
	return &traceContextHandler{Handler: h.Handler.WithGroup(name)}
}

// LoggerFromContext returns a logger that includes trace context from ctx.
// If no logger is stored in context, returns the default logger.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// ContextWithLogger returns a new context with the logger stored.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

type loggerKey struct{}
