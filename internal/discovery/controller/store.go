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

package controller

import (
	"context"
	"fmt"
	"sync"
)

// MemoryCatalogStore is an in-memory implementation of CatalogStore for testing.
type MemoryCatalogStore struct {
	mu    sync.RWMutex
	items map[string]*DiscoveredItem
}

// NewMemoryCatalogStore creates a new in-memory catalog store.
func NewMemoryCatalogStore() *MemoryCatalogStore {
	return &MemoryCatalogStore{
		items: make(map[string]*DiscoveredItem),
	}
}

// itemKey generates a unique key for an item.
func itemKey(componentName, version string) string {
	return fmt.Sprintf("%s@%s", componentName, version)
}

// CreateOrUpdate creates or updates a catalog item.
func (s *MemoryCatalogStore) CreateOrUpdate(ctx context.Context, item *DiscoveredItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := itemKey(item.ComponentName, item.Version)
	s.items[key] = item
	return nil
}

// Get gets a catalog item by component name and version.
func (s *MemoryCatalogStore) Get(ctx context.Context, componentName, version string) (*DiscoveredItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := itemKey(componentName, version)
	item, ok := s.items[key]
	if !ok {
		return nil, fmt.Errorf("item not found: %s", key)
	}
	return item, nil
}

// List lists all catalog items.
func (s *MemoryCatalogStore) List(ctx context.Context) ([]*DiscoveredItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]*DiscoveredItem, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	return items, nil
}

// Count returns the number of items in the store.
func (s *MemoryCatalogStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// Clear removes all items from the store.
func (s *MemoryCatalogStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = make(map[string]*DiscoveredItem)
}
