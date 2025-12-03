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

// Package registry provides REST storage implementations for Solar API resources.
package registry

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/registry/rest"
)

// MemoryStore is a simple in-memory storage implementation.
// This is primarily for development/testing; production should use etcd.
type MemoryStore struct {
	mu           sync.RWMutex
	items        map[string]runtime.Object
	watchers     map[int]*memoryWatcher
	watcherCount int
	newFunc      func() runtime.Object
	newListFunc  func() runtime.Object
	gvk          schema.GroupVersionKind
	namespaced   bool
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore(
	newFunc func() runtime.Object,
	newListFunc func() runtime.Object,
	gvk schema.GroupVersionKind,
	namespaced bool,
) *MemoryStore {
	return &MemoryStore{
		items:       make(map[string]runtime.Object),
		watchers:    make(map[int]*memoryWatcher),
		newFunc:     newFunc,
		newListFunc: newListFunc,
		gvk:         gvk,
		namespaced:  namespaced,
	}
}

// key generates a storage key for an object.
func (s *MemoryStore) key(namespace, name string) string {
	if s.namespaced {
		return fmt.Sprintf("%s/%s", namespace, name)
	}
	return name
}

// New returns a new instance of the resource.
func (s *MemoryStore) New() runtime.Object {
	return s.newFunc()
}

// Destroy cleans up resources.
func (s *MemoryStore) Destroy() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.watchers {
		w.Stop()
	}
	s.watchers = make(map[int]*memoryWatcher)
}

// NewList returns a new list instance.
func (s *MemoryStore) NewList() runtime.Object {
	return s.newListFunc()
}

// NamespaceScoped returns true if the resource is namespaced.
func (s *MemoryStore) NamespaceScoped() bool {
	return s.namespaced
}

// GetSingularName returns the singular name of the resource.
func (s *MemoryStore) GetSingularName() string {
	return s.gvk.Kind
}

// Get retrieves an object by namespace and name.
func (s *MemoryStore) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	namespace := genericNamespace(ctx)
	key := s.key(namespace, name)

	obj, exists := s.items[key]
	if !exists {
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    s.gvk.Group,
			Resource: s.gvk.Kind,
		}, name)
	}

	return obj.DeepCopyObject(), nil
}

// List returns a list of objects matching the options.
func (s *MemoryStore) List(
	ctx context.Context,
	options *metav1.ListOptions,
) (runtime.Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	namespace := genericNamespace(ctx)
	list := s.newListFunc()

	var items []runtime.Object
	for k, v := range s.items {
		// Filter by namespace if namespaced.
		if s.namespaced && namespace != "" {
			accessor, err := meta.Accessor(v)
			if err != nil {
				continue
			}
			if accessor.GetNamespace() != namespace {
				continue
			}
		}
		_ = k // key not needed for filtering currently
		items = append(items, v.DeepCopyObject())
	}

	if err := meta.SetList(list, items); err != nil {
		return nil, err
	}

	return list, nil
}

// Create stores a new object.
func (s *MemoryStore) Create(
	ctx context.Context,
	obj runtime.Object,
	createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions,
) (runtime.Object, error) {
	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	namespace := accessor.GetNamespace()
	name := accessor.GetName()

	if name == "" {
		return nil, errors.NewBadRequest("name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.key(namespace, name)

	if _, exists := s.items[key]; exists {
		return nil, errors.NewAlreadyExists(schema.GroupResource{
			Group:    s.gvk.Group,
			Resource: s.gvk.Kind,
		}, name)
	}

	// Set creation timestamp and resource version.
	now := metav1.Now()
	accessor.SetCreationTimestamp(now)
	accessor.SetResourceVersion("1")
	accessor.SetUID(types.UID(generateUID()))

	s.items[key] = obj.DeepCopyObject()

	// Notify watchers.
	s.notifyWatchers(watch.Added, obj)

	return obj.DeepCopyObject(), nil
}

// Update updates an existing object.
func (s *MemoryStore) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions,
) (runtime.Object, bool, error) {
	namespace := genericNamespace(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.key(namespace, name)
	existing, exists := s.items[key]

	var oldObj runtime.Object
	if exists {
		oldObj = existing.DeepCopyObject()
	} else if !forceAllowCreate {
		return nil, false, errors.NewNotFound(schema.GroupResource{
			Group:    s.gvk.Group,
			Resource: s.gvk.Kind,
		}, name)
	}

	newObj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}

	if !exists {
		// Create new object.
		if createValidation != nil {
			if err := createValidation(ctx, newObj); err != nil {
				return nil, false, err
			}
		}

		accessor, err := meta.Accessor(newObj)
		if err != nil {
			return nil, false, err
		}

		now := metav1.Now()
		accessor.SetCreationTimestamp(now)
		accessor.SetResourceVersion("1")
		accessor.SetUID(types.UID(generateUID()))

		s.items[key] = newObj.DeepCopyObject()
		s.notifyWatchers(watch.Added, newObj)

		return newObj.DeepCopyObject(), true, nil
	}

	// Update existing object.
	if updateValidation != nil {
		if err := updateValidation(ctx, newObj, oldObj); err != nil {
			return nil, false, err
		}
	}

	accessor, err := meta.Accessor(newObj)
	if err != nil {
		return nil, false, err
	}

	oldAccessor, _ := meta.Accessor(oldObj)
	rv := oldAccessor.GetResourceVersion()
	newRV := incrementResourceVersion(rv)
	accessor.SetResourceVersion(newRV)

	s.items[key] = newObj.DeepCopyObject()
	s.notifyWatchers(watch.Modified, newObj)

	return newObj.DeepCopyObject(), false, nil
}

// Delete removes an object.
func (s *MemoryStore) Delete(
	ctx context.Context,
	name string,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
) (runtime.Object, bool, error) {
	namespace := genericNamespace(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.key(namespace, name)
	obj, exists := s.items[key]
	if !exists {
		return nil, false, errors.NewNotFound(schema.GroupResource{
			Group:    s.gvk.Group,
			Resource: s.gvk.Kind,
		}, name)
	}

	if deleteValidation != nil {
		if err := deleteValidation(ctx, obj); err != nil {
			return nil, false, err
		}
	}

	delete(s.items, key)
	s.notifyWatchers(watch.Deleted, obj)

	return obj.DeepCopyObject(), true, nil
}

// DeleteCollection deletes multiple objects.
func (s *MemoryStore) DeleteCollection(
	ctx context.Context,
	deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions,
	listOptions *metav1.ListOptions,
) (runtime.Object, error) {
	list, err := s.List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		accessor, err := meta.Accessor(item)
		if err != nil {
			continue
		}
		_, _, _ = s.Delete(ctx, accessor.GetName(), deleteValidation, options)
	}

	return list, nil
}

// Watch returns a watch.Interface that watches the requested objects.
func (s *MemoryStore) Watch(ctx context.Context, options *metav1.ListOptions) (watch.Interface, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w := newMemoryWatcher()
	s.watcherCount++
	id := s.watcherCount
	s.watchers[id] = w

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		delete(s.watchers, id)
		s.mu.Unlock()
		w.Stop()
	}()

	return w, nil
}

func (s *MemoryStore) notifyWatchers(eventType watch.EventType, obj runtime.Object) {
	for _, w := range s.watchers {
		w.Send(watch.Event{
			Type:   eventType,
			Object: obj.DeepCopyObject(),
		})
	}
}

// memoryWatcher implements watch.Interface for in-memory storage.
type memoryWatcher struct {
	ch     chan watch.Event
	done   chan struct{}
	closed bool
	mu     sync.Mutex
}

func newMemoryWatcher() *memoryWatcher {
	return &memoryWatcher{
		ch:   make(chan watch.Event, 100),
		done: make(chan struct{}),
	}
}

func (w *memoryWatcher) ResultChan() <-chan watch.Event {
	return w.ch
}

func (w *memoryWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		w.closed = true
		close(w.done)
		close(w.ch)
	}
}

func (w *memoryWatcher) Send(event watch.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	select {
	case w.ch <- event:
	default:
		// Drop events if buffer is full.
	}
}

// Helper functions

func genericNamespace(ctx context.Context) string {
	if ns, ok := ctx.Value(namespaceKey).(string); ok {
		return ns
	}
	// Try to get from request info.
	return ""
}

type contextKey string

const namespaceKey contextKey = "namespace"

func generateUID() string {
	// Simple UUID v4 generation.
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		randUint32(), randUint16(), randUint16(), randUint16(), randUint48())
}

var (
	randMu    sync.Mutex
	randState uint64 = 1
)

func randUint32() uint32 {
	randMu.Lock()
	defer randMu.Unlock()
	randState = randState*6364136223846793005 + 1
	return uint32(randState >> 32)
}

func randUint16() uint16 {
	return uint16(randUint32() >> 16)
}

func randUint48() uint64 {
	return uint64(randUint32())<<16 | uint64(randUint16())
}

func incrementResourceVersion(rv string) string {
	var v int
	fmt.Sscanf(rv, "%d", &v)
	return fmt.Sprintf("%d", v+1)
}
