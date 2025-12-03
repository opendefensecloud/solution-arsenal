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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

// StatusStore provides storage for the /status subresource.
type StatusStore struct {
	store       *MemoryStore
	copyStatus  func(from, to runtime.Object)
	newFunc     func() runtime.Object
	namespaced  bool
	singularName string
}

// NewStatusStore creates a new StatusStore.
func NewStatusStore(
	store *MemoryStore,
	copyStatus func(from, to runtime.Object),
	newFunc func() runtime.Object,
	namespaced bool,
	singularName string,
) *StatusStore {
	return &StatusStore{
		store:       store,
		copyStatus:  copyStatus,
		newFunc:     newFunc,
		namespaced:  namespaced,
		singularName: singularName,
	}
}

// New returns a new instance of the resource.
func (s *StatusStore) New() runtime.Object {
	return s.newFunc()
}

// Destroy cleans up resources.
func (s *StatusStore) Destroy() {
	// Nothing to clean up; the main store handles this.
}

// Get retrieves an object by namespace and name.
func (s *StatusStore) Get(
	ctx context.Context,
	name string,
	options *metav1.GetOptions,
) (runtime.Object, error) {
	return s.store.Get(ctx, name, options)
}

// Update updates only the status of an existing object.
func (s *StatusStore) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions,
) (runtime.Object, bool, error) {
	// Wrap the update to only copy the status field.
	statusObjInfo := &statusUpdateInfo{
		UpdatedObjectInfo: objInfo,
		copyStatus:        s.copyStatus,
	}

	return s.store.Update(ctx, name, statusObjInfo, createValidation, updateValidation, false, options)
}

// GetSingularName returns the singular name of the resource.
func (s *StatusStore) GetSingularName() string {
	return s.singularName
}

// statusUpdateInfo wraps UpdatedObjectInfo to only update status.
type statusUpdateInfo struct {
	rest.UpdatedObjectInfo
	copyStatus func(from, to runtime.Object)
}

// UpdatedObject returns the object with only the status updated.
func (s *statusUpdateInfo) UpdatedObject(ctx context.Context, oldObj runtime.Object) (runtime.Object, error) {
	newObj, err := s.UpdatedObjectInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, err
	}

	// Copy the old object and only update the status from the new object.
	result := oldObj.DeepCopyObject()
	s.copyStatus(newObj, result)

	return result, nil
}
