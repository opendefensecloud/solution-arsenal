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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

func TestWatchManager_Watch(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "TestResource"}
	wm := NewWatchManager(gvk, 10)
	defer wm.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := wm.Watch(ctx)
	require.NotNil(t, w)

	stats := wm.Stats()
	assert.Equal(t, int64(1), stats.ActiveWatchers)
}

func TestWatchManager_Broadcast(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "TestResource"}
	wm := NewWatchManager(gvk, 10)
	defer wm.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := wm.Watch(ctx)

	// Broadcast an event
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
	}

	wm.Broadcast(watch.Added, obj)

	// Should receive the event
	select {
	case event := <-w.ResultChan():
		assert.Equal(t, watch.Added, event.Type)
		cm := event.Object.(*corev1.ConfigMap)
		assert.Equal(t, "test", cm.Name)
	case <-time.After(time.Second):
		t.Fatal("Expected to receive event")
	}

	stats := wm.Stats()
	assert.Equal(t, int64(1), stats.TotalEvents)
}

func TestWatchManager_MultipleWatchers(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "TestResource"}
	wm := NewWatchManager(gvk, 10)
	defer wm.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w1 := wm.Watch(ctx)
	w2 := wm.Watch(ctx)
	w3 := wm.Watch(ctx)

	stats := wm.Stats()
	assert.Equal(t, int64(3), stats.ActiveWatchers)

	// Broadcast
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}
	wm.Broadcast(watch.Added, obj)

	// All watchers should receive
	for _, w := range []watch.Interface{w1, w2, w3} {
		select {
		case event := <-w.ResultChan():
			assert.Equal(t, watch.Added, event.Type)
		case <-time.After(time.Second):
			t.Fatal("Expected to receive event")
		}
	}

	stats = wm.Stats()
	assert.Equal(t, int64(3), stats.TotalEvents)
}

func TestWatchManager_WatcherStop(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "TestResource"}
	wm := NewWatchManager(gvk, 10)
	defer wm.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := wm.Watch(ctx)

	stats := wm.Stats()
	assert.Equal(t, int64(1), stats.ActiveWatchers)

	w.Stop()

	// Give time for cleanup
	time.Sleep(10 * time.Millisecond)

	stats = wm.Stats()
	assert.Equal(t, int64(0), stats.ActiveWatchers)
}

func TestWatchManager_ContextCancellation(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "TestResource"}
	wm := NewWatchManager(gvk, 10)
	defer wm.Close()

	ctx, cancel := context.WithCancel(context.Background())

	_ = wm.Watch(ctx)

	stats := wm.Stats()
	assert.Equal(t, int64(1), stats.ActiveWatchers)

	// Cancel context
	cancel()

	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)

	stats = wm.Stats()
	assert.Equal(t, int64(0), stats.ActiveWatchers)
}

func TestWatchManager_DroppedEvents(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "TestResource"}
	wm := NewWatchManager(gvk, 2) // Small buffer
	defer wm.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = wm.Watch(ctx) // Don't consume events

	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}

	// Send more events than buffer size
	for i := 0; i < 5; i++ {
		wm.Broadcast(watch.Added, obj)
	}

	stats := wm.Stats()
	// 2 should succeed (buffer size), 3 should be dropped
	assert.Equal(t, int64(2), stats.TotalEvents)
	assert.Equal(t, int64(3), stats.DroppedEvents)
}

func TestFilteredWatcher(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "TestResource"}
	wm := NewWatchManager(gvk, 10)
	defer wm.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inner := wm.Watch(ctx)

	// Filter only Added events
	filter := func(e watch.Event) bool {
		return e.Type == watch.Added
	}

	fw := NewFilteredWatcher(inner, filter, 10)
	defer fw.Stop()

	// Send different event types
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}

	wm.Broadcast(watch.Added, obj)
	wm.Broadcast(watch.Modified, obj)
	wm.Broadcast(watch.Added, obj)
	wm.Broadcast(watch.Deleted, obj)

	// Should only receive Added events
	received := 0
	timeout := time.After(500 * time.Millisecond)

loop:
	for {
		select {
		case event, ok := <-fw.ResultChan():
			if !ok {
				break loop
			}
			assert.Equal(t, watch.Added, event.Type)
			received++
			if received == 2 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}

	assert.Equal(t, 2, received)
}

func TestWatchManager_Close(t *testing.T) {
	t.Parallel()

	gvk := schema.GroupVersionKind{Group: "test", Version: "v1", Kind: "TestResource"}
	wm := NewWatchManager(gvk, 10)

	ctx := context.Background()

	w1 := wm.Watch(ctx)
	w2 := wm.Watch(ctx)

	stats := wm.Stats()
	assert.Equal(t, int64(2), stats.ActiveWatchers)

	wm.Close()

	// Channels should be closed
	_, ok1 := <-w1.ResultChan()
	_, ok2 := <-w2.ResultChan()
	assert.False(t, ok1)
	assert.False(t, ok2)
}
