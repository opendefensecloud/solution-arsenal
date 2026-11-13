// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func TestReleaseBindingSpecChangedPredicate(t *testing.T) {
	p := releaseBindingSpecChangedPredicate()
	old := &solarv1alpha1.ReleaseBinding{Name: "b", Generation: 1}

	statusOnly := old.DeepCopy()
	statusOnly.Status.Conditions = []metav1.Condition{{Type: solarv1alpha1.ConditionTypeDeployed}}
	if p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: statusOnly}) {
		t.Fatal("a status-only update must not pass")
	}

	specChange := old.DeepCopy()
	specChange.Generation = 2
	if !p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: specChange}) {
		t.Fatal("a generation change must pass")
	}

	deleting := old.DeepCopy()
	deleting.DeletionTimestamp = new(metav1.Now())
	if !p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: deleting}) {
		t.Fatal("a deletion timestamp change must pass")
	}

	if !p.Create(event.CreateEvent{Object: old}) || !p.Delete(event.DeleteEvent{Object: old}) {
		t.Fatal("creates and deletes must pass")
	}
}

func TestTargetResolvedBindingsChangedPredicate(t *testing.T) {
	p := targetResolvedBindingsChangedPredicate()
	old := &solarv1alpha1.Target{Name: "t"}

	if !p.Create(event.CreateEvent{Object: old}) {
		t.Fatal("a create must pass")
	}

	conditionsOnly := old.DeepCopy()
	conditionsOnly.Status.Conditions = []metav1.Condition{{Type: "ReleasesRendered"}}
	if p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: conditionsOnly}) {
		t.Fatal("an update without a resolvedBindings change must not pass")
	}

	listed := old.DeepCopy()
	listed.Status.ResolvedBindings = []solarv1alpha1.ResolvedBinding{{Namespace: "ns", Name: "b"}}
	if !p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: listed}) {
		t.Fatal("a resolvedBindings change must pass")
	}

	if p.Delete(event.DeleteEvent{Object: old}) {
		t.Fatal("a delete must not pass: the report is garbage-collected with its Target")
	}
}

func TestRenderArtifactDigestChangedPredicate(t *testing.T) {
	p := renderArtifactDigestChangedPredicate()
	old := &solarv1alpha1.RenderArtifact{Name: "a"}

	urlOnly := old.DeepCopy()
	urlOnly.Status.ChartURL = "oci://registry.example.com/ns/a:v1"
	if p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: urlOnly}) {
		t.Fatal("an update without a digest change must not pass")
	}

	digested := old.DeepCopy()
	digested.Status.Digest = "sha256:abc"
	if !p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: digested}) {
		t.Fatal("a digest change must pass")
	}

	if p.Create(event.CreateEvent{Object: digested}) || p.Delete(event.DeleteEvent{Object: digested}) {
		t.Fatal("creates and deletes must not pass: the Target lists an artifact before it has a digest")
	}
}
