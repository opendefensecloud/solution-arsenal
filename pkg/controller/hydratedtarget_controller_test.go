// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"encoding/json"
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"go.opendefense.cloud/kit/envtest"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/renderer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func validHydratedTarget(name string, namespace *corev1.Namespace) *solarv1alpha1.HydratedTarget {
	return &solarv1alpha1.HydratedTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace.Name,
		},
		Spec: solarv1alpha1.HydratedTargetSpec{
			Releases: map[string]corev1.LocalObjectReference{
				"my-release": {
					Name: "my-release",
				},
			},
			Userdata: runtime.RawExtension{
				Raw: []byte(`{"key": "value"}`),
			},
		},
	}
}

var _ = Describe("HydratedTargetReconciler", Ordered, func() {
	var (
		ctx       = envtest.Context()
		namespace = setupTest(ctx)
	)

	BeforeEach(func() {
		// Create the referenced Release
		rel := validRelease("my-release", namespace)
		rel.Status.ChartURL = fmt.Sprintf("oci://%s/my-release:v0.0.0", namespace.Name)
		Expect(k8sClient.Create(ctx, rel)).To(Succeed())
		Expect(k8sClient.Status().Patch(ctx, rel, client.MergeFrom(rel))).To(Succeed())
	})

	Describe("HydratedTarget creation and job scheduling", func() {
		It("should create a HydratedTarget and schedule a renderer job", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Verify the HydratedTarget was created
			createdHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht", Namespace: namespace.Name}, createdHT)
			}).Should(Succeed())

			// Verify finalizer was added after a reconciliation cycle
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht", Namespace: namespace.Name}, createdHT)
				if err != nil {
					return false
				}
				return len(createdHT.Finalizers) > 0 && slices.Contains(createdHT.Finalizers, RenderJobFinalizer)
			}, eventuallyTimeout).Should(BeTrue(), "finalizer should be added by reconciler")

			// Verify config secret was created
			configSecret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-config", Namespace: namespace.Name}, configSecret)
			}, eventuallyTimeout).Should(Succeed())

			Expect(configSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(configSecret.Data).To(HaveKey("config.json"))

			// Verify job was created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("renderer"))
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal("image:tag"))
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(ContainElement("solar-renderer"))

			// Verify config secret is mounted
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Volumes[0].Name).To(Equal("config"))
			Expect(job.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal("test-ht-config"))
		})

		It("should create a HydratedTarget and fill the config secret correctly", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Verify the HydratedTarget was created
			createdHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht", Namespace: namespace.Name}, createdHT)
			}).Should(Succeed())

			configSecret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-config", Namespace: namespace.Name}, configSecret)
			}, eventuallyTimeout).Should(Succeed())

			Expect(configSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(configSecret.Data).To(HaveKey("config.json"))

			jsonData := configSecret.Data["config.json"]
			rendererConfig := &renderer.Config{}
			Expect(json.Unmarshal(jsonData, rendererConfig)).To(Succeed())

			Expect(rendererConfig.Type).To(Equal(renderer.TypeHydratedTarget))
			Expect(rendererConfig.PushOptions.ReferenceURL).To(Equal(fmt.Sprintf("oci://%s/ht-%s:v0.0.0", namespace.Name, ht.Name)))

			Expect(rendererConfig.HydratedTargetConfig.Chart.Name).To(Equal(ht.Name))
			Expect(rendererConfig.HydratedTargetConfig.Chart.Version).NotTo(BeEmpty())

			Expect(rendererConfig.HydratedTargetConfig.Input.Releases).NotTo(BeNil())
			ra := rendererConfig.HydratedTargetConfig.Input.Releases["my-release"]
			Expect(ra).NotTo(BeNil())
			Expect(ra.Tag).To(Equal("v0.0.0"))
			Expect(ra.Repository).To(Equal(fmt.Sprintf("%s/my-release", namespace.Name)))
		})

		It("should set JobScheduled condition when job is running", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-running", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-running-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Wait for JobScheduled condition
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-running", Namespace: namespace.Name}, updatedHT)
				if err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedHT.Status.Conditions, ConditionTypeJobScheduled)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify the condition
			condition := apimeta.FindStatusCondition(updatedHT.Status.Conditions, ConditionTypeJobScheduled)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("JobScheduled"))
		})

		It("should not recreate a job if one already exists", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-existing", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-existing-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Get the job's UID
			originalUID := job.UID

			// Wait a bit to ensure no new jobs are created
			Consistently(func() string {
				updatedJob := &batchv1.Job{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-existing-renderer", Namespace: namespace.Name}, updatedJob); err != nil {
					return ""
				}
				return string(updatedJob.UID)
			}, "2s", pollingInterval).Should(Equal(string(originalUID)))
		})
	})

	Describe("HydratedTarget job completion and cleanup", func() {
		It("should cleanup job and secret when job completes successfully", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-success", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-success-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Simulate job completion by updating its status
			job.Status.Succeeded = 1
			now := metav1.Now()
			job.Status.StartTime = &now
			job.Status.CompletionTime = &now

			// Set the SuccessCriteriaMet condition
			successCondition := batchv1.JobCondition{
				Type:               batchv1.JobSuccessCriteriaMet,
				Status:             corev1.ConditionTrue,
				LastProbeTime:      now,
				LastTransitionTime: now,
			}
			job.Status.Conditions = append(job.Status.Conditions, successCondition)

			// Set the Complete condition
			jobCondition := batchv1.JobCondition{
				Type:               batchv1.JobComplete,
				Status:             corev1.ConditionTrue,
				LastProbeTime:      now,
				LastTransitionTime: now,
			}
			job.Status.Conditions = append(job.Status.Conditions, jobCondition)
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			// Wait for HydratedTarget to get JobSucceeded condition
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-success", Namespace: namespace.Name}, updatedHT)
				if err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedHT.Status.Conditions, ConditionTypeJobSucceeded)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify JobSucceeded condition
			condition := apimeta.FindStatusCondition(updatedHT.Status.Conditions, ConditionTypeJobSucceeded)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("JobSucceeded"))

			// Verify job is deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-success-renderer", Namespace: namespace.Name}, job)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())

			// Verify config secret is deleted
			secret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-success-config", Namespace: namespace.Name}, secret)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should not recreate resources after successful completion", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-stable", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-stable-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Simulate job completion
			job.Status.Succeeded = 1
			now := metav1.Now()
			job.Status.StartTime = &now
			job.Status.CompletionTime = &now

			// Set the SuccessCriteriaMet condition
			successCondition := batchv1.JobCondition{
				Type:               batchv1.JobSuccessCriteriaMet,
				Status:             corev1.ConditionTrue,
				LastProbeTime:      now,
				LastTransitionTime: now,
			}
			job.Status.Conditions = append(job.Status.Conditions, successCondition)

			// Set the Complete condition
			jobCondition := batchv1.JobCondition{
				Type:               batchv1.JobComplete,
				Status:             corev1.ConditionTrue,
				LastProbeTime:      now,
				LastTransitionTime: now,
			}
			job.Status.Conditions = append(job.Status.Conditions, jobCondition)
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			// Wait for resources to be cleaned up and HydratedTarget to show success
			Eventually(func() bool {
				updatedHT := &solarv1alpha1.HydratedTarget{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-stable", Namespace: namespace.Name}, updatedHT); err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedHT.Status.Conditions, ConditionTypeJobSucceeded)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify resources are deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-stable-renderer", Namespace: namespace.Name}, job)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())

			secret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-stable-config", Namespace: namespace.Name}, secret)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())

			// Wait and verify they are not recreated
			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-stable-renderer", Namespace: namespace.Name}, job)
				return client.IgnoreNotFound(err) == nil
			}, "2s", pollingInterval).Should(BeTrue())
		})

		It("should set JobFailed condition when job fails", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-failed", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-failed-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Simulate job failure by updating its status
			job.Status.Failed = 1
			now := metav1.Now()
			job.Status.StartTime = &now
			// Don't set completionTime for failed jobs - just mark as failed

			// Set the FailureTarget condition
			failureTargetCondition := batchv1.JobCondition{
				Type:               batchv1.JobFailureTarget,
				Status:             corev1.ConditionTrue,
				LastProbeTime:      now,
				LastTransitionTime: now,
			}
			job.Status.Conditions = append(job.Status.Conditions, failureTargetCondition)

			// Set the Failed condition
			failedCondition := batchv1.JobCondition{
				Type:               batchv1.JobFailed,
				Status:             corev1.ConditionTrue,
				LastProbeTime:      now,
				LastTransitionTime: now,
			}
			job.Status.Conditions = append(job.Status.Conditions, failedCondition)
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			// Wait for HydratedTarget to get JobFailed condition
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-failed", Namespace: namespace.Name}, updatedHT)
				if err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedHT.Status.Conditions, ConditionTypeJobFailed)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify JobFailed condition
			condition := apimeta.FindStatusCondition(updatedHT.Status.Conditions, ConditionTypeJobFailed)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("JobFailed"))

			// Verify job and secret still exist when failed (not cleaned up)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-failed-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-failed-config", Namespace: namespace.Name}, secret)).To(Succeed())
		})
	})

	Describe("HydratedTarget deletion", func() {
		It("should cleanup resources when HydratedTarget is deleted", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-delete", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for job and secret to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete-renderer", Namespace: namespace.Name}, job)
			}).Should(Succeed())

			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete-config", Namespace: namespace.Name}, secret)
			}).Should(Succeed())

			// Delete the HydratedTarget
			createdHT := &solarv1alpha1.HydratedTarget{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete", Namespace: namespace.Name}, createdHT)).To(Succeed())
			Expect(k8sClient.Delete(ctx, createdHT)).To(Succeed())

			// Verify job is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete-renderer", Namespace: namespace.Name}, job)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify secret is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete-config", Namespace: namespace.Name}, secret)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify HydratedTarget is deleted (finalizer removed)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-delete", Namespace: namespace.Name}, createdHT)
			}).Should(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("HydratedTarget status references", func() {
		It("should maintain references to created job and secret in HydratedTarget status", func() {
			// Create a HydratedTarget
			ht := validHydratedTarget("test-ht-refs", namespace)
			Expect(k8sClient.Create(ctx, ht)).To(Succeed())

			// Wait for job and secret to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-refs-renderer", Namespace: namespace.Name}, job)
			}).Should(Succeed())

			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-refs-config", Namespace: namespace.Name}, secret)
			}).Should(Succeed())

			// Verify HydratedTarget status has references
			updatedHT := &solarv1alpha1.HydratedTarget{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-ht-refs", Namespace: namespace.Name}, updatedHT)
				if err != nil {
					return false
				}
				return updatedHT.Status.JobRef != nil && updatedHT.Status.ConfigSecretRef != nil
			}).Should(BeTrue())

			// Verify JobRef details
			Expect(updatedHT.Status.JobRef.Name).To(Equal("test-ht-refs-renderer"))
			Expect(updatedHT.Status.JobRef.Namespace).To(Equal(namespace.Name))
			Expect(updatedHT.Status.JobRef.Kind).To(Equal("Job"))
			Expect(updatedHT.Status.JobRef.APIVersion).To(Equal("batch/v1"))

			// Verify ConfigSecretRef details
			Expect(updatedHT.Status.ConfigSecretRef.Name).To(Equal("test-ht-refs-config"))
			Expect(updatedHT.Status.ConfigSecretRef.Namespace).To(Equal(namespace.Name))
			Expect(updatedHT.Status.ConfigSecretRef.Kind).To(Equal("Secret"))
			Expect(updatedHT.Status.ConfigSecretRef.APIVersion).To(Equal("v1"))
		})
	})
})
