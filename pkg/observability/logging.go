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

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerConfig holds configuration for the logger.
type LoggerConfig struct {
	// Level is the log level (debug, info, warn, error).
	Level string
	// Development enables development mode with more verbose output.
	Development bool
	// Encoding is the log encoding (json or console).
	Encoding string
}

// NewLogger creates a new logr.Logger backed by zap.
func NewLogger(cfg LoggerConfig) (logr.Logger, error) {
	var zapCfg zap.Config

	if cfg.Development {
		zapCfg = zap.NewDevelopmentConfig()
	} else {
		zapCfg = zap.NewProductionConfig()
	}

	// Set encoding.
	if cfg.Encoding != "" {
		zapCfg.Encoding = cfg.Encoding
	}

	// Set log level.
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}
	zapCfg.Level = zap.NewAtomicLevelAt(level)

	// Build the logger.
	zapLog, err := zapCfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		return logr.Discard(), err
	}

	return zapr.NewLogger(zapLog), nil
}

// LoggerWithTraceContext returns a logger enriched with trace context from the given context.
// This enables log correlation with distributed traces.
func LoggerWithTraceContext(ctx context.Context, logger logr.Logger) logr.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return logger
	}

	spanCtx := span.SpanContext()
	return logger.WithValues(
		"trace_id", spanCtx.TraceID().String(),
		"span_id", spanCtx.SpanID().String(),
	)
}

// LoggerWithValues returns a logger with additional key-value pairs.
func LoggerWithValues(logger logr.Logger, keysAndValues ...interface{}) logr.Logger {
	return logger.WithValues(keysAndValues...)
}

// LoggerWithName returns a logger with the given name.
func LoggerWithName(logger logr.Logger, name string) logr.Logger {
	return logger.WithName(name)
}

// loggerKey is the context key for storing the logger.
type loggerKey struct{}

// ContextWithLogger returns a new context with the logger attached.
func ContextWithLogger(ctx context.Context, logger logr.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// LoggerFromContext returns the logger from the context.
// If no logger is found, it returns a discard logger.
func LoggerFromContext(ctx context.Context) logr.Logger {
	logger, ok := ctx.Value(loggerKey{}).(logr.Logger)
	if !ok {
		return logr.Discard()
	}
	return logger
}

// LoggerFromContextOrDefault returns the logger from the context,
// or the default logger if none is found.
func LoggerFromContextOrDefault(ctx context.Context, defaultLogger logr.Logger) logr.Logger {
	logger, ok := ctx.Value(loggerKey{}).(logr.Logger)
	if !ok {
		return defaultLogger
	}
	return logger
}
