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

package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"

	solarv1alpha1 "github.com/opendefensecloud/solution-arsenal/pkg/apis/solar/v1alpha1"
)

// contextWithNamespace creates a context with the namespace set.
func contextWithNamespace(ctx context.Context, ns string) context.Context {
	return context.WithValue(ctx, namespaceKey, ns)
}

func newTestStore() *MemoryStore {
	return NewMemoryStore(
		func() runtime.Object { return &solarv1alpha1.CatalogItem{} },
		func() runtime.Object { return &solarv1alpha1.CatalogItemList{} },
		solarv1alpha1.GroupVersion.WithKind("CatalogItem"),
		true,
	)
}

func TestMemoryStore_Create(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	defer store.Destroy()

	ctx := context.Background()
	obj := &solarv1alpha1.CatalogItem{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-item",
			Namespace: "default",
		},
		Spec: solarv1alpha1.CatalogItemSpec{
			ComponentName: "test-component",
			Version:       "1.0.0",
			Repository:    "ghcr.io/test/repo",
		},
	}

	created, err := store.Create(ctx, obj, nil, &metav1.CreateOptions{})
	require.NoError(t, err)
	require.NotNil(t, created)

	item := created.(*solarv1alpha1.CatalogItem)
	assert.Equal(t, "test-item", item.Name)
	assert.Equal(t, "default", item.Namespace)
	assert.NotEmpty(t, item.UID)
	assert.NotEmpty(t, item.ResourceVersion)
	assert.False(t, item.CreationTimestamp.IsZero())
}

func TestMemoryStore_Create_AlreadyExists(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	defer store.Destroy()

	ctx := context.Background()
	obj := &solarv1alpha1.CatalogItem{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-item",
			Namespace: "default",
		},
	}

	_, err := store.Create(ctx, obj, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	// Try to create again.
	_, err = store.Create(ctx, obj, nil, &metav1.CreateOptions{})
	require.Error(t, err)
	assert.True(t, errors.IsAlreadyExists(err))
}

func TestMemoryStore_Get(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	defer store.Destroy()

	ctx := contextWithNamespace(context.Background(), "default")
	obj := &solarv1alpha1.CatalogItem{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-item",
			Namespace: "default",
		},
		Spec: solarv1alpha1.CatalogItemSpec{
			ComponentName: "test-component",
		},
	}

	_, err := store.Create(ctx, obj, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	// Get the item.
	got, err := store.Get(ctx, "test-item", &metav1.GetOptions{})
	require.NoError(t, err)

	item := got.(*solarv1alpha1.CatalogItem)
	assert.Equal(t, "test-item", item.Name)
	assert.Equal(t, "test-component", item.Spec.ComponentName)
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	defer store.Destroy()

	ctx := context.Background()
	_, err := store.Get(ctx, "nonexistent", &metav1.GetOptions{})
	require.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestMemoryStore_List(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	defer store.Destroy()

	ctx := context.Background()

	// Create multiple items.
	for i := 0; i < 3; i++ {
		obj := &solarv1alpha1.CatalogItem{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "item-" + string(rune('a'+i)),
				Namespace: "default",
			},
		}
		_, err := store.Create(ctx, obj, nil, &metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// List all items.
	list, err := store.List(ctx, &metav1.ListOptions{})
	require.NoError(t, err)

	itemList := list.(*solarv1alpha1.CatalogItemList)
	assert.Len(t, itemList.Items, 3)
}

func TestMemoryStore_Delete(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	defer store.Destroy()

	ctx := contextWithNamespace(context.Background(), "default")
	obj := &solarv1alpha1.CatalogItem{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-item",
			Namespace: "default",
		},
	}

	_, err := store.Create(ctx, obj, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	// Delete the item.
	deleted, wasDeleted, err := store.Delete(ctx, "test-item", nil, &metav1.DeleteOptions{})
	require.NoError(t, err)
	assert.True(t, wasDeleted)
	assert.NotNil(t, deleted)

	// Verify it's gone.
	_, err = store.Get(ctx, "test-item", &metav1.GetOptions{})
	assert.True(t, errors.IsNotFound(err))
}

func TestMemoryStore_Delete_NotFound(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	defer store.Destroy()

	ctx := context.Background()
	_, _, err := store.Delete(ctx, "nonexistent", nil, &metav1.DeleteOptions{})
	require.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestMemoryStore_Update(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	defer store.Destroy()

	ctx := contextWithNamespace(context.Background(), "default")
	obj := &solarv1alpha1.CatalogItem{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-item",
			Namespace: "default",
		},
		Spec: solarv1alpha1.CatalogItemSpec{
			ComponentName: "original",
		},
	}

	created, err := store.Create(ctx, obj, nil, &metav1.CreateOptions{})
	require.NoError(t, err)
	originalRV := created.(*solarv1alpha1.CatalogItem).ResourceVersion

	// Update the item.
	updateInfo := &simpleUpdateInfo{
		obj: &solarv1alpha1.CatalogItem{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-item",
				Namespace: "default",
			},
			Spec: solarv1alpha1.CatalogItemSpec{
				ComponentName: "updated",
			},
		},
	}

	updated, created2, err := store.Update(ctx, "test-item", updateInfo, nil, nil, false, &metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.False(t, created2)

	item := updated.(*solarv1alpha1.CatalogItem)
	assert.Equal(t, "updated", item.Spec.ComponentName)
	assert.NotEqual(t, originalRV, item.ResourceVersion)
}

func TestMemoryStore_Watch(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	defer store.Destroy()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watching.
	watcher, err := store.Watch(ctx, &metav1.ListOptions{})
	require.NoError(t, err)
	defer watcher.Stop()

	// Create an item.
	obj := &solarv1alpha1.CatalogItem{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-item",
			Namespace: "default",
		},
	}

	_, err = store.Create(context.Background(), obj, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	// Should receive an event.
	select {
	case event := <-watcher.ResultChan():
		assert.Equal(t, watch.Added, event.Type)
		item := event.Object.(*solarv1alpha1.CatalogItem)
		assert.Equal(t, "test-item", item.Name)
	default:
		t.Fatal("Expected to receive watch event")
	}
}

func TestMemoryStore_NamespaceScoped(t *testing.T) {
	t.Parallel()

	namespacedStore := NewMemoryStore(
		func() runtime.Object { return &solarv1alpha1.CatalogItem{} },
		func() runtime.Object { return &solarv1alpha1.CatalogItemList{} },
		solarv1alpha1.GroupVersion.WithKind("CatalogItem"),
		true,
	)
	assert.True(t, namespacedStore.NamespaceScoped())

	clusterStore := NewMemoryStore(
		func() runtime.Object { return &solarv1alpha1.ClusterCatalogItem{} },
		func() runtime.Object { return &solarv1alpha1.ClusterCatalogItemList{} },
		solarv1alpha1.GroupVersion.WithKind("ClusterCatalogItem"),
		false,
	)
	assert.False(t, clusterStore.NamespaceScoped())
}

func TestMemoryStore_New(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	obj := store.New()
	assert.NotNil(t, obj)
	_, ok := obj.(*solarv1alpha1.CatalogItem)
	assert.True(t, ok)
}

func TestMemoryStore_NewList(t *testing.T) {
	t.Parallel()

	store := newTestStore()
	obj := store.NewList()
	assert.NotNil(t, obj)
	_, ok := obj.(*solarv1alpha1.CatalogItemList)
	assert.True(t, ok)
}

// simpleUpdateInfo is a simple implementation of rest.UpdatedObjectInfo for testing.
type simpleUpdateInfo struct {
	obj runtime.Object
}

func (s *simpleUpdateInfo) Preconditions() *metav1.Preconditions {
	return nil
}

func (s *simpleUpdateInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error) {
	return s.obj.DeepCopyObject(), nil
}
