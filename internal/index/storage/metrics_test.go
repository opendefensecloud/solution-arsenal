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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics(t *testing.T) {
	t.Parallel()

	metrics, err := NewMetrics()
	require.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.operationDuration)
	assert.NotNil(t, metrics.operationCount)
	assert.NotNil(t, metrics.operationErrors)
	assert.NotNil(t, metrics.activeWatches)
	assert.NotNil(t, metrics.watchEvents)
	assert.NotNil(t, metrics.watchEventsDropped)
	assert.NotNil(t, metrics.objectCount)
	assert.NotNil(t, metrics.objectSize)
}

func TestMetrics_RecordOperation(t *testing.T) {
	t.Parallel()

	metrics, err := NewMetrics()
	require.NoError(t, err)

	ctx := context.Background()

	// Test successful operation
	metrics.RecordOperation(ctx, "catalogitems", "create", 100*time.Millisecond, nil)

	// Test failed operation
	metrics.RecordOperation(ctx, "catalogitems", "get", 50*time.Millisecond, errors.New("not found"))
}

func TestMetrics_RecordWatch(t *testing.T) {
	t.Parallel()

	metrics, err := NewMetrics()
	require.NoError(t, err)

	ctx := context.Background()

	// Start watch
	metrics.RecordWatchStart(ctx, "releases")

	// Record events
	metrics.RecordWatchEvent(ctx, "releases", "ADDED")
	metrics.RecordWatchEvent(ctx, "releases", "MODIFIED")
	metrics.RecordWatchEventDropped(ctx, "releases")

	// Stop watch
	metrics.RecordWatchStop(ctx, "releases")
}

func TestMetrics_RecordObjects(t *testing.T) {
	t.Parallel()

	metrics, err := NewMetrics()
	require.NoError(t, err)

	ctx := context.Background()

	// Create objects
	metrics.RecordObjectCreated(ctx, "clusterregistrations", 1024)
	metrics.RecordObjectCreated(ctx, "clusterregistrations", 2048)

	// Delete object
	metrics.RecordObjectDeleted(ctx, "clusterregistrations")
}

func TestOperationTimer(t *testing.T) {
	t.Parallel()

	metrics, err := NewMetrics()
	require.NoError(t, err)

	ctx := context.Background()

	// Test with timer
	timer := metrics.StartOperationTimer("syncs", "update")
	time.Sleep(10 * time.Millisecond)
	timer.Done(ctx, nil)

	// Test timer with error
	timer = metrics.StartOperationTimer("syncs", "delete")
	time.Sleep(5 * time.Millisecond)
	timer.Done(ctx, errors.New("failed"))
}

func TestNoopMetrics(t *testing.T) {
	t.Parallel()

	metrics := NoopMetrics()
	assert.NotNil(t, metrics)

	// Should not panic
	ctx := context.Background()
	metrics.RecordOperation(ctx, "test", "op", time.Second, nil)
	metrics.RecordWatchStart(ctx, "test")
	metrics.RecordWatchStop(ctx, "test")
}
