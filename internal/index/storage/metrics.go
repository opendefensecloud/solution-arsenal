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

package storage

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	meterName = "solar.odc.io/storage"
)

// Metrics holds storage-related metrics.
type Metrics struct {
	// Operation metrics
	operationDuration metric.Float64Histogram
	operationCount    metric.Int64Counter
	operationErrors   metric.Int64Counter

	// Watch metrics
	activeWatches      metric.Int64UpDownCounter
	watchEvents        metric.Int64Counter
	watchEventsDropped metric.Int64Counter

	// Storage metrics
	objectCount metric.Int64UpDownCounter
	objectSize  metric.Int64Histogram
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter(meterName)

	operationDuration, err := meter.Float64Histogram(
		"solar_storage_operation_duration_seconds",
		metric.WithDescription("Duration of storage operations in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	operationCount, err := meter.Int64Counter(
		"solar_storage_operations_total",
		metric.WithDescription("Total number of storage operations"),
	)
	if err != nil {
		return nil, err
	}

	operationErrors, err := meter.Int64Counter(
		"solar_storage_operation_errors_total",
		metric.WithDescription("Total number of storage operation errors"),
	)
	if err != nil {
		return nil, err
	}

	activeWatches, err := meter.Int64UpDownCounter(
		"solar_storage_active_watches",
		metric.WithDescription("Number of active watch connections"),
	)
	if err != nil {
		return nil, err
	}

	watchEvents, err := meter.Int64Counter(
		"solar_storage_watch_events_total",
		metric.WithDescription("Total number of watch events sent"),
	)
	if err != nil {
		return nil, err
	}

	watchEventsDropped, err := meter.Int64Counter(
		"solar_storage_watch_events_dropped_total",
		metric.WithDescription("Total number of watch events dropped"),
	)
	if err != nil {
		return nil, err
	}

	objectCount, err := meter.Int64UpDownCounter(
		"solar_storage_objects",
		metric.WithDescription("Number of objects in storage by resource type"),
	)
	if err != nil {
		return nil, err
	}

	objectSize, err := meter.Int64Histogram(
		"solar_storage_object_size_bytes",
		metric.WithDescription("Size of stored objects in bytes"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		operationDuration:  operationDuration,
		operationCount:     operationCount,
		operationErrors:    operationErrors,
		activeWatches:      activeWatches,
		watchEvents:        watchEvents,
		watchEventsDropped: watchEventsDropped,
		objectCount:        objectCount,
		objectSize:         objectSize,
	}, nil
}

// RecordOperation records a storage operation.
func (m *Metrics) RecordOperation(ctx context.Context, resource, operation string, duration time.Duration, err error) {
	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
		attribute.String("operation", operation),
	}

	m.operationCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.operationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if err != nil {
		m.operationErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordWatchStart records a new watch connection.
func (m *Metrics) RecordWatchStart(ctx context.Context, resource string) {
	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
	}
	m.activeWatches.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordWatchStop records a watch connection closing.
func (m *Metrics) RecordWatchStop(ctx context.Context, resource string) {
	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
	}
	m.activeWatches.Add(ctx, -1, metric.WithAttributes(attrs...))
}

// RecordWatchEvent records a watch event being sent.
func (m *Metrics) RecordWatchEvent(ctx context.Context, resource, eventType string) {
	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
		attribute.String("event_type", eventType),
	}
	m.watchEvents.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordWatchEventDropped records a watch event being dropped.
func (m *Metrics) RecordWatchEventDropped(ctx context.Context, resource string) {
	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
	}
	m.watchEventsDropped.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordObjectCreated records an object being created.
func (m *Metrics) RecordObjectCreated(ctx context.Context, resource string, sizeBytes int64) {
	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
	}
	m.objectCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.objectSize.Record(ctx, sizeBytes, metric.WithAttributes(attrs...))
}

// RecordObjectDeleted records an object being deleted.
func (m *Metrics) RecordObjectDeleted(ctx context.Context, resource string) {
	attrs := []attribute.KeyValue{
		attribute.String("resource", resource),
	}
	m.objectCount.Add(ctx, -1, metric.WithAttributes(attrs...))
}

// OperationTimer is a helper for timing operations.
type OperationTimer struct {
	metrics   *Metrics
	resource  string
	operation string
	start     time.Time
}

// StartOperationTimer creates a new operation timer.
func (m *Metrics) StartOperationTimer(resource, operation string) *OperationTimer {
	return &OperationTimer{
		metrics:   m,
		resource:  resource,
		operation: operation,
		start:     time.Now(),
	}
}

// Done records the operation completion.
func (t *OperationTimer) Done(ctx context.Context, err error) {
	t.metrics.RecordOperation(ctx, t.resource, t.operation, time.Since(t.start), err)
}

// NoopMetrics returns a no-op metrics instance for testing.
func NoopMetrics() *Metrics {
	m, _ := NewMetrics()
	return m
}
