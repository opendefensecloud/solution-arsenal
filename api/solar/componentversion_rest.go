// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"go.opendefense.cloud/kit/apiserver/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ resource.Object = &ComponentVersion{}

func (o *ComponentVersion) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *ComponentVersion) NamespaceScoped() bool {
	return true
}

func (o *ComponentVersion) New() runtime.Object {
	return &ComponentVersion{}
}

func (o *ComponentVersion) NewList() runtime.Object {
	return &ComponentVersionList{}
}

func (o *ComponentVersion) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("componentversions").GroupResource()
}
