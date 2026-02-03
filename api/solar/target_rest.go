// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"go.opendefense.cloud/kit/apiserver/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var _ resource.Object = &Target{}

func (o *Target) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *Target) NamespaceScoped() bool {
	return true
}

func (o *Target) New() runtime.Object {
	return &Target{}
}

func (o *Target) NewList() runtime.Object {
	return &TargetList{}
}

func (o *Target) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("targets").GroupResource()
}
