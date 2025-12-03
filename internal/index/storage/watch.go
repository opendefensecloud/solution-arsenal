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

// Package storage provides storage utilities for the solar-index API server.
package storage

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"
)

// WatchManager manages watches for a storage backend.
type WatchManager struct {
	mu           sync.RWMutex
	watchers     map[int64]*managedWatcher
	watcherCount int64
	gvk          schema.GroupVersionKind
	bufferSize   int

	// Metrics
	activeWatchers  int64
	totalEvents     int64
	droppedEvents   int64
}

// NewWatchManager creates a new watch manager.
func NewWatchManager(gvk schema.GroupVersionKind, bufferSize int) *WatchManager {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	return &WatchManager{
		watchers:   make(map[int64]*managedWatcher),
		gvk:        gvk,
		bufferSize: bufferSize,
	}
}

// Watch creates a new watch and registers it with the manager.
func (m *WatchManager) Watch(ctx context.Context) watch.Interface {
	m.mu.Lock()

	id := atomic.AddInt64(&m.watcherCount, 1)
	w := newManagedWatcher(id, m.bufferSize, m.onWatcherClose)
	m.watchers[id] = w
	atomic.AddInt64(&m.activeWatchers, 1)

	klog.V(4).InfoS("Watch created",
		"resource", m.gvk.Kind,
		"watcherID", id,
		"activeWatchers", atomic.LoadInt64(&m.activeWatchers))

	m.mu.Unlock()

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		w.Stop()
	}()

	return w
}

// onWatcherClose is called when a watcher is stopped.
func (m *WatchManager) onWatcherClose(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.watchers, id)
	atomic.AddInt64(&m.activeWatchers, -1)

	klog.V(4).InfoS("Watch closed",
		"resource", m.gvk.Kind,
		"watcherID", id,
		"activeWatchers", atomic.LoadInt64(&m.activeWatchers))
}

// Broadcast sends an event to all registered watchers.
func (m *WatchManager) Broadcast(eventType watch.EventType, obj runtime.Object) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, w := range m.watchers {
		if w.Send(watch.Event{Type: eventType, Object: obj.DeepCopyObject()}) {
			atomic.AddInt64(&m.totalEvents, 1)
		} else {
			atomic.AddInt64(&m.droppedEvents, 1)
		}
	}
}

// Close stops all watchers and cleans up resources.
func (m *WatchManager) Close() {
	m.mu.Lock()
	// Copy watchers to avoid holding lock during Stop
	watchers := make([]*managedWatcher, 0, len(m.watchers))
	for _, w := range m.watchers {
		watchers = append(watchers, w)
	}
	m.watchers = make(map[int64]*managedWatcher)
	m.mu.Unlock()

	// Stop watchers without holding the lock
	for _, w := range watchers {
		w.stopWithoutCallback()
	}
	atomic.StoreInt64(&m.activeWatchers, 0)
}

// Stats returns current watch statistics.
func (m *WatchManager) Stats() WatchStats {
	return WatchStats{
		ActiveWatchers: atomic.LoadInt64(&m.activeWatchers),
		TotalEvents:    atomic.LoadInt64(&m.totalEvents),
		DroppedEvents:  atomic.LoadInt64(&m.droppedEvents),
	}
}

// WatchStats contains statistics about watches.
type WatchStats struct {
	ActiveWatchers int64
	TotalEvents    int64
	DroppedEvents  int64
}

// managedWatcher is a watcher managed by WatchManager.
type managedWatcher struct {
	id       int64
	ch       chan watch.Event
	done     chan struct{}
	closed   bool
	mu       sync.Mutex
	onClose  func(int64)
	lastSend time.Time
}

// newManagedWatcher creates a new managed watcher.
func newManagedWatcher(id int64, bufferSize int, onClose func(int64)) *managedWatcher {
	return &managedWatcher{
		id:      id,
		ch:      make(chan watch.Event, bufferSize),
		done:    make(chan struct{}),
		onClose: onClose,
	}
}

// ResultChan returns the channel for receiving events.
func (w *managedWatcher) ResultChan() <-chan watch.Event {
	return w.ch
}

// Stop stops the watcher and calls the onClose callback.
func (w *managedWatcher) Stop() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}

	w.closed = true
	close(w.done)
	close(w.ch)
	onClose := w.onClose
	id := w.id
	w.mu.Unlock()

	// Call callback outside of lock to avoid deadlock
	if onClose != nil {
		onClose(id)
	}
}

// stopWithoutCallback stops the watcher without calling onClose.
// Used by WatchManager.Close to avoid deadlock.
func (w *managedWatcher) stopWithoutCallback() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return
	}

	w.closed = true
	close(w.done)
	close(w.ch)
}

// Send sends an event to the watcher.
// Returns true if the event was sent, false if it was dropped.
func (w *managedWatcher) Send(event watch.Event) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return false
	}

	select {
	case w.ch <- event:
		w.lastSend = time.Now()
		return true
	default:
		// Buffer full, drop event
		klog.V(2).InfoS("Watch event dropped due to full buffer",
			"watcherID", w.id,
			"eventType", event.Type)
		return false
	}
}

// BookmarkWatcher wraps a watcher to add bookmark support.
type BookmarkWatcher struct {
	inner           watch.Interface
	bookmarkCh      chan watch.Event
	resourceVersion string
	mu              sync.Mutex
}

// NewBookmarkWatcher creates a watcher that sends periodic bookmarks.
func NewBookmarkWatcher(inner watch.Interface, bookmarkInterval time.Duration) *BookmarkWatcher {
	bw := &BookmarkWatcher{
		inner:      inner,
		bookmarkCh: make(chan watch.Event, 1),
	}

	// Bookmarks are handled by the underlying storage in production
	// This is mainly for testing and development

	return bw
}

// ResultChan returns the channel for receiving events.
func (bw *BookmarkWatcher) ResultChan() <-chan watch.Event {
	return bw.inner.ResultChan()
}

// Stop stops the watcher.
func (bw *BookmarkWatcher) Stop() {
	bw.inner.Stop()
}

// SetResourceVersion updates the resource version for bookmarks.
func (bw *BookmarkWatcher) SetResourceVersion(rv string) {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	bw.resourceVersion = rv
}

// FilteredWatcher wraps a watcher to filter events.
type FilteredWatcher struct {
	inner  watch.Interface
	filter func(watch.Event) bool
	ch     chan watch.Event
	done   chan struct{}
}

// NewFilteredWatcher creates a watcher that filters events.
func NewFilteredWatcher(inner watch.Interface, filter func(watch.Event) bool, bufferSize int) *FilteredWatcher {
	if bufferSize <= 0 {
		bufferSize = 100
	}

	fw := &FilteredWatcher{
		inner:  inner,
		filter: filter,
		ch:     make(chan watch.Event, bufferSize),
		done:   make(chan struct{}),
	}

	go fw.run()
	return fw
}

func (fw *FilteredWatcher) run() {
	defer close(fw.ch)

	for {
		select {
		case <-fw.done:
			return
		case event, ok := <-fw.inner.ResultChan():
			if !ok {
				return
			}
			if fw.filter == nil || fw.filter(event) {
				select {
				case fw.ch <- event:
				case <-fw.done:
					return
				}
			}
		}
	}
}

// ResultChan returns the filtered event channel.
func (fw *FilteredWatcher) ResultChan() <-chan watch.Event {
	return fw.ch
}

// Stop stops the filtered watcher.
func (fw *FilteredWatcher) Stop() {
	close(fw.done)
	fw.inner.Stop()
}
