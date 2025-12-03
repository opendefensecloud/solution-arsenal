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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCatalogItem_IsAvailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    CatalogItemPhase
		expected bool
	}{
		{"available", CatalogItemPhaseAvailable, true},
		{"unavailable", CatalogItemPhaseUnavailable, false},
		{"deprecated", CatalogItemPhaseDeprecated, false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &CatalogItem{Status: CatalogItemStatus{Phase: tt.phase}}
			assert.Equal(t, tt.expected, c.IsAvailable())
		})
	}
}

func TestCatalogItem_IsDeprecated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    CatalogItemPhase
		expected bool
	}{
		{"deprecated", CatalogItemPhaseDeprecated, true},
		{"available", CatalogItemPhaseAvailable, false},
		{"unavailable", CatalogItemPhaseUnavailable, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &CatalogItem{Status: CatalogItemStatus{Phase: tt.phase}}
			assert.Equal(t, tt.expected, c.IsDeprecated())
		})
	}
}

func TestCatalogItem_GetCondition(t *testing.T) {
	t.Parallel()

	c := &CatalogItem{
		Status: CatalogItemStatus{
			Conditions: []metav1.Condition{
				{Type: CatalogItemConditionTypeReady, Status: metav1.ConditionTrue},
				{Type: CatalogItemConditionTypeAvailable, Status: metav1.ConditionFalse},
			},
		},
	}

	t.Run("existing condition", func(t *testing.T) {
		t.Parallel()
		cond := c.GetCondition(CatalogItemConditionTypeReady)
		assert.NotNil(t, cond)
		assert.Equal(t, metav1.ConditionTrue, cond.Status)
	})

	t.Run("non-existing condition", func(t *testing.T) {
		t.Parallel()
		cond := c.GetCondition("NonExistent")
		assert.Nil(t, cond)
	})
}

func TestCatalogItem_SetCondition(t *testing.T) {
	t.Parallel()

	t.Run("add new condition", func(t *testing.T) {
		t.Parallel()
		c := &CatalogItem{}
		c.SetCondition(metav1.Condition{
			Type:    CatalogItemConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonSucceeded,
			Message: "Ready",
		})

		assert.Len(t, c.Status.Conditions, 1)
		assert.Equal(t, CatalogItemConditionTypeReady, c.Status.Conditions[0].Type)
		assert.Equal(t, metav1.ConditionTrue, c.Status.Conditions[0].Status)
	})

	t.Run("update existing condition with status change", func(t *testing.T) {
		t.Parallel()
		c := &CatalogItem{
			Status: CatalogItemStatus{
				Conditions: []metav1.Condition{
					{Type: CatalogItemConditionTypeReady, Status: metav1.ConditionFalse},
				},
			},
		}

		c.SetCondition(metav1.Condition{
			Type:    CatalogItemConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonSucceeded,
			Message: "Now ready",
		})

		assert.Len(t, c.Status.Conditions, 1)
		assert.Equal(t, metav1.ConditionTrue, c.Status.Conditions[0].Status)
		assert.Equal(t, "Now ready", c.Status.Conditions[0].Message)
	})

	t.Run("update existing condition without status change", func(t *testing.T) {
		t.Parallel()
		c := &CatalogItem{
			Status: CatalogItemStatus{
				Conditions: []metav1.Condition{
					{Type: CatalogItemConditionTypeReady, Status: metav1.ConditionTrue, Message: "Old message"},
				},
			},
		}

		c.SetCondition(metav1.Condition{
			Type:    CatalogItemConditionTypeReady,
			Status:  metav1.ConditionTrue,
			Reason:  ReasonSucceeded,
			Message: "New message",
		})

		assert.Len(t, c.Status.Conditions, 1)
		assert.Equal(t, "New message", c.Status.Conditions[0].Message)
	})
}

func TestClusterRegistration_IsReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    ClusterPhase
		expected bool
	}{
		{"ready", ClusterPhaseReady, true},
		{"pending", ClusterPhasePending, false},
		{"connecting", ClusterPhaseConnecting, false},
		{"not ready", ClusterPhaseNotReady, false},
		{"unreachable", ClusterPhaseUnreachable, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &ClusterRegistration{Status: ClusterRegistrationStatus{Phase: tt.phase}}
			assert.Equal(t, tt.expected, c.IsReady())
		})
	}
}

func TestClusterRegistration_IsConnected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    ClusterPhase
		expected bool
	}{
		{"ready", ClusterPhaseReady, true},
		{"not ready", ClusterPhaseNotReady, true},
		{"pending", ClusterPhasePending, false},
		{"connecting", ClusterPhaseConnecting, false},
		{"unreachable", ClusterPhaseUnreachable, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &ClusterRegistration{Status: ClusterRegistrationStatus{Phase: tt.phase}}
			assert.Equal(t, tt.expected, c.IsConnected())
		})
	}
}

func TestClusterRegistration_HasRelease(t *testing.T) {
	t.Parallel()

	c := &ClusterRegistration{
		Status: ClusterRegistrationStatus{
			InstalledReleases: []ReleaseReference{
				{Name: "release-1", Version: "1.0.0"},
				{Name: "release-2", Version: "2.0.0"},
			},
		},
	}

	assert.True(t, c.HasRelease("release-1"))
	assert.True(t, c.HasRelease("release-2"))
	assert.False(t, c.HasRelease("release-3"))
}

func TestRelease_IsDeployed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    ReleasePhase
		expected bool
	}{
		{"deployed", ReleasePhaseDeployed, true},
		{"pending", ReleasePhasePending, false},
		{"rendering", ReleasePhaseRendering, false},
		{"deploying", ReleasePhaseDeploying, false},
		{"failed", ReleasePhaseFailed, false},
		{"suspended", ReleasePhaseSuspended, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &Release{Status: ReleaseStatus{Phase: tt.phase}}
			assert.Equal(t, tt.expected, r.IsDeployed())
		})
	}
}

func TestRelease_IsSuspended(t *testing.T) {
	t.Parallel()

	t.Run("spec suspend true", func(t *testing.T) {
		t.Parallel()
		r := &Release{Spec: ReleaseSpec{Suspend: true}}
		assert.True(t, r.IsSuspended())
	})

	t.Run("status suspended", func(t *testing.T) {
		t.Parallel()
		r := &Release{Status: ReleaseStatus{Phase: ReleasePhaseSuspended}}
		assert.True(t, r.IsSuspended())
	})

	t.Run("not suspended", func(t *testing.T) {
		t.Parallel()
		r := &Release{Status: ReleaseStatus{Phase: ReleasePhaseDeployed}}
		assert.False(t, r.IsSuspended())
	})
}

func TestRelease_IsFailed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    ReleasePhase
		expected bool
	}{
		{"failed", ReleasePhaseFailed, true},
		{"deployed", ReleasePhaseDeployed, false},
		{"pending", ReleasePhasePending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &Release{Status: ReleaseStatus{Phase: tt.phase}}
			assert.Equal(t, tt.expected, r.IsFailed())
		})
	}
}

func TestSync_IsSynced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    SyncPhase
		expected bool
	}{
		{"synced", SyncPhaseSynced, true},
		{"pending", SyncPhasePending, false},
		{"syncing", SyncPhaseSyncing, false},
		{"failed", SyncPhaseFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Sync{Status: SyncStatus{Phase: tt.phase}}
			assert.Equal(t, tt.expected, s.IsSynced())
		})
	}
}

func TestSync_IsFailed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		phase    SyncPhase
		expected bool
	}{
		{"failed", SyncPhaseFailed, true},
		{"synced", SyncPhaseSynced, false},
		{"syncing", SyncPhaseSyncing, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &Sync{Status: SyncStatus{Phase: tt.phase}}
			assert.Equal(t, tt.expected, s.IsFailed())
		})
	}
}
