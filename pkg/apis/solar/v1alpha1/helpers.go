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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CatalogItem helpers

// IsAvailable returns true if the CatalogItem is in Available phase.
func (c *CatalogItem) IsAvailable() bool {
	return c.Status.Phase == CatalogItemPhaseAvailable
}

// IsDeprecated returns true if the CatalogItem is deprecated.
func (c *CatalogItem) IsDeprecated() bool {
	return c.Status.Phase == CatalogItemPhaseDeprecated
}

// GetCondition returns the condition with the given type, or nil if not found.
func (c *CatalogItem) GetCondition(conditionType string) *metav1.Condition {
	for i := range c.Status.Conditions {
		if c.Status.Conditions[i].Type == conditionType {
			return &c.Status.Conditions[i]
		}
	}
	return nil
}

// SetCondition sets a condition on the CatalogItem, replacing any existing condition of the same type.
func (c *CatalogItem) SetCondition(condition metav1.Condition) {
	condition.LastTransitionTime = metav1.Now()
	for i := range c.Status.Conditions {
		if c.Status.Conditions[i].Type == condition.Type {
			if c.Status.Conditions[i].Status != condition.Status {
				c.Status.Conditions[i] = condition
			} else {
				// Only update message/reason if status hasn't changed
				c.Status.Conditions[i].Message = condition.Message
				c.Status.Conditions[i].Reason = condition.Reason
			}
			return
		}
	}
	c.Status.Conditions = append(c.Status.Conditions, condition)
}

// ClusterCatalogItem helpers

// IsAvailable returns true if the ClusterCatalogItem is in Available phase.
func (c *ClusterCatalogItem) IsAvailable() bool {
	return c.Status.Phase == CatalogItemPhaseAvailable
}

// GetCondition returns the condition with the given type, or nil if not found.
func (c *ClusterCatalogItem) GetCondition(conditionType string) *metav1.Condition {
	for i := range c.Status.Conditions {
		if c.Status.Conditions[i].Type == conditionType {
			return &c.Status.Conditions[i]
		}
	}
	return nil
}

// SetCondition sets a condition on the ClusterCatalogItem.
func (c *ClusterCatalogItem) SetCondition(condition metav1.Condition) {
	condition.LastTransitionTime = metav1.Now()
	for i := range c.Status.Conditions {
		if c.Status.Conditions[i].Type == condition.Type {
			if c.Status.Conditions[i].Status != condition.Status {
				c.Status.Conditions[i] = condition
			} else {
				c.Status.Conditions[i].Message = condition.Message
				c.Status.Conditions[i].Reason = condition.Reason
			}
			return
		}
	}
	c.Status.Conditions = append(c.Status.Conditions, condition)
}

// ClusterRegistration helpers

// IsReady returns true if the cluster is ready for deployments.
func (c *ClusterRegistration) IsReady() bool {
	return c.Status.Phase == ClusterPhaseReady
}

// IsConnected returns true if the agent is connected.
func (c *ClusterRegistration) IsConnected() bool {
	return c.Status.Phase == ClusterPhaseReady || c.Status.Phase == ClusterPhaseNotReady
}

// GetCondition returns the condition with the given type, or nil if not found.
func (c *ClusterRegistration) GetCondition(conditionType string) *metav1.Condition {
	for i := range c.Status.Conditions {
		if c.Status.Conditions[i].Type == conditionType {
			return &c.Status.Conditions[i]
		}
	}
	return nil
}

// SetCondition sets a condition on the ClusterRegistration.
func (c *ClusterRegistration) SetCondition(condition metav1.Condition) {
	condition.LastTransitionTime = metav1.Now()
	for i := range c.Status.Conditions {
		if c.Status.Conditions[i].Type == condition.Type {
			if c.Status.Conditions[i].Status != condition.Status {
				c.Status.Conditions[i] = condition
			} else {
				c.Status.Conditions[i].Message = condition.Message
				c.Status.Conditions[i].Reason = condition.Reason
			}
			return
		}
	}
	c.Status.Conditions = append(c.Status.Conditions, condition)
}

// HasRelease returns true if the cluster has the given release installed.
func (c *ClusterRegistration) HasRelease(name string) bool {
	for _, r := range c.Status.InstalledReleases {
		if r.Name == name {
			return true
		}
	}
	return false
}

// Release helpers

// IsDeployed returns true if the release is deployed.
func (r *Release) IsDeployed() bool {
	return r.Status.Phase == ReleasePhaseDeployed
}

// IsSuspended returns true if the release is suspended.
func (r *Release) IsSuspended() bool {
	return r.Spec.Suspend || r.Status.Phase == ReleasePhaseSuspended
}

// IsFailed returns true if the release has failed.
func (r *Release) IsFailed() bool {
	return r.Status.Phase == ReleasePhaseFailed
}

// GetCondition returns the condition with the given type, or nil if not found.
func (r *Release) GetCondition(conditionType string) *metav1.Condition {
	for i := range r.Status.Conditions {
		if r.Status.Conditions[i].Type == conditionType {
			return &r.Status.Conditions[i]
		}
	}
	return nil
}

// SetCondition sets a condition on the Release.
func (r *Release) SetCondition(condition metav1.Condition) {
	condition.LastTransitionTime = metav1.Now()
	for i := range r.Status.Conditions {
		if r.Status.Conditions[i].Type == condition.Type {
			if r.Status.Conditions[i].Status != condition.Status {
				r.Status.Conditions[i] = condition
			} else {
				r.Status.Conditions[i].Message = condition.Message
				r.Status.Conditions[i].Reason = condition.Reason
			}
			return
		}
	}
	r.Status.Conditions = append(r.Status.Conditions, condition)
}

// Sync helpers

// IsSynced returns true if the sync has completed successfully.
func (s *Sync) IsSynced() bool {
	return s.Status.Phase == SyncPhaseSynced
}

// IsFailed returns true if the sync has failed.
func (s *Sync) IsFailed() bool {
	return s.Status.Phase == SyncPhaseFailed
}

// GetCondition returns the condition with the given type, or nil if not found.
func (s *Sync) GetCondition(conditionType string) *metav1.Condition {
	for i := range s.Status.Conditions {
		if s.Status.Conditions[i].Type == conditionType {
			return &s.Status.Conditions[i]
		}
	}
	return nil
}

// SetCondition sets a condition on the Sync.
func (s *Sync) SetCondition(condition metav1.Condition) {
	condition.LastTransitionTime = metav1.Now()
	for i := range s.Status.Conditions {
		if s.Status.Conditions[i].Type == condition.Type {
			if s.Status.Conditions[i].Status != condition.Status {
				s.Status.Conditions[i] = condition
			} else {
				s.Status.Conditions[i].Message = condition.Message
				s.Status.Conditions[i].Reason = condition.Reason
			}
			return
		}
	}
	s.Status.Conditions = append(s.Status.Conditions, condition)
}
