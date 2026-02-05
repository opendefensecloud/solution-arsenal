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

func validRelease(name string, namespace *corev1.Namespace) *solarv1alpha1.Release {
	return &solarv1alpha1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace.Name,
		},
		Spec: solarv1alpha1.ReleaseSpec{
			ComponentVersionRef: corev1.LocalObjectReference{
				Name: "my-component-v1",
			},
			Values: runtime.RawExtension{
				Raw: []byte(`{"key": "value"}`),
			},
		},
	}
}

func validComponentVersion(name string, namespace *corev1.Namespace) *solarv1alpha1.ComponentVersion {
	return &solarv1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace.Name,
		},
		Spec: solarv1alpha1.ComponentVersionSpec{
			ComponentRef: corev1.LocalObjectReference{
				Name: "my-component",
			},
			Tag: "v1.0.0",
			Helm: solarv1alpha1.ResourceAccess{
				Repository: "example.com/helm",
				Tag:        "1.0.1",
			},
			KRO: solarv1alpha1.ResourceAccess{
				Repository: "example.com/kro",
				Tag:        "^1.0",
			},
			Resources: map[string]solarv1alpha1.ResourceAccess{
				"foo": {Repository: "example.com/resources/foo", Tag: "2.0.0"},
				"bar": {Repository: "example.com/resources/bar", Tag: "3.0.0"},
			},
		},
	}
}

var _ = Describe("ReleaseReconciler", Ordered, func() {
	var (
		ctx       = envtest.Context()
		namespace = setupTest(ctx)
	)

	BeforeEach(func() {
		// Create the Componentversion
		cv := validComponentVersion("my-component-v1", namespace)
		Expect(k8sClient.Create(ctx, cv)).To(Succeed())
	})

	Describe("Release creation and job scheduling", func() {
		It("should create a Release and schedule a renderer job", func() {
			// Create a Release
			release := validRelease("test-release", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Verify the Release was created
			createdRelease := &solarv1alpha1.Release{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release", Namespace: namespace.Name}, createdRelease)
			}).Should(Succeed())

			// Verify finalizer was added after a reconciliation cycle
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release", Namespace: namespace.Name}, createdRelease)
				if err != nil {
					return false
				}
				return len(createdRelease.Finalizers) > 0 && slices.Contains(createdRelease.Finalizers, RenderJobFinalizer)
			}, eventuallyTimeout).Should(BeTrue(), "finalizer should be added by reconciler") // Verify config secret was created
			configSecret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-config", Namespace: namespace.Name}, configSecret)
			}, eventuallyTimeout).Should(Succeed())

			Expect(configSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(configSecret.Data).To(HaveKey("config.json"))

			// Verify job was created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("renderer"))
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal("image:tag"))
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(ContainElement("solar-renderer"))

			// Verify config secret is mounted
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Volumes[0].Name).To(Equal("config"))
			Expect(job.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal("test-release-config"))
		})

		It("should create a Release and fill the config secret correctly", func() {
			// Create a Release
			release := validRelease("test-release", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for the secret to be created
			configSecret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-config", Namespace: namespace.Name}, configSecret)
			}, eventuallyTimeout).Should(Succeed())

			Expect(configSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(configSecret.Data).To(HaveKey("config.json"))

			jsonData := configSecret.Data["config.json"]
			rendererConfig := &renderer.Config{}
			Expect(json.Unmarshal(jsonData, rendererConfig)).To(Succeed())

			Expect(rendererConfig.Type).To(Equal(renderer.TypeRelease))
			Expect(rendererConfig.HydratedTargetConfig).To(BeZero())
			Expect(rendererConfig.ReleaseConfig.Chart.Version).To(Equal("v0.0.0"))
			// FIXME: Check puhsoptions
			// Expect(rendererConfig.PushOptions.ReferenceURL).To(Equal("myregistry.local/myrelease"))

			Expect(rendererConfig.ReleaseConfig.Values).NotTo(BeNil())
			Expect(rendererConfig.ReleaseConfig.Values).To(BeEquivalentTo(release.Spec.Values.Raw))

			cv := validComponentVersion("my-component-v1", namespace)
			Expect(rendererConfig.ReleaseConfig.Input.Component.Name).To(Equal(cv.Spec.ComponentRef.Name))
			Expect(rendererConfig.ReleaseConfig.Input.KRO).To(Equal(cv.Spec.KRO))
			Expect(rendererConfig.ReleaseConfig.Input.Helm).To(Equal(cv.Spec.Helm))
			Expect(rendererConfig.ReleaseConfig.Input.Resources).NotTo(BeNil())
			Expect(rendererConfig.ReleaseConfig.Input.Resources).To(Equal(cv.Spec.Resources))
		})

		It("should set the ChartURL status field", func() {
			// Create a Release
			release := validRelease("test-release-status", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for the secret to be created
			configSecret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-status-config", Namespace: namespace.Name}, configSecret)
			}, eventuallyTimeout).Should(Succeed())

			updatedRelease := &solarv1alpha1.Release{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-status", Namespace: namespace.Name}, updatedRelease)).To(Succeed())
			url := updatedRelease.Status.ChartURL

			Expect(url).To(Equal(fmt.Sprintf("oci://%s/rel-test-release-status:v0.0.0", namespace.Name)))
		})

		It("should set JobScheduled condition when job is running", func() {
			// Create a Release
			release := validRelease("test-release-running", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-running-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Wait for JobScheduled condition
			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-running", Namespace: namespace.Name}, updatedRelease)
				if err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedRelease.Status.Conditions, ConditionTypeJobScheduled)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify the condition
			condition := apimeta.FindStatusCondition(updatedRelease.Status.Conditions, ConditionTypeJobScheduled)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("JobScheduled"))
		})

		It("should not recreate a job if one already exists", func() {
			// Create a Release
			release := validRelease("test-release-existing", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-existing-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Get the job's UID
			originalUID := job.UID

			// Wait a bit to ensure no new jobs are created
			Consistently(func() string {
				updatedJob := &batchv1.Job{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-existing-renderer", Namespace: namespace.Name}, updatedJob); err != nil {
					return ""
				}
				return string(updatedJob.UID)
			}, "2s", pollingInterval).Should(Equal(string(originalUID)))
		})
	})

	Describe("Release job completion and cleanup", func() {
		It("should cleanup job and secret when job completes successfully", func() {
			// Create a Release
			release := validRelease("test-release-success", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-success-renderer", Namespace: namespace.Name}, job)
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

			// Wait for Release to get JobSucceeded condition
			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-success", Namespace: namespace.Name}, updatedRelease)
				if err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedRelease.Status.Conditions, ConditionTypeJobSucceeded)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify JobSucceeded condition
			condition := apimeta.FindStatusCondition(updatedRelease.Status.Conditions, ConditionTypeJobSucceeded)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("JobSucceeded"))

			// Verify job is deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-success-renderer", Namespace: namespace.Name}, job)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())

			// Verify config secret is deleted
			secret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-success-config", Namespace: namespace.Name}, secret)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should not recreate resources after successful completion", func() {
			// Create a Release
			release := validRelease("test-release-stable", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-stable-renderer", Namespace: namespace.Name}, job)
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

			// Wait for resources to be cleaned up and Release to show success
			Eventually(func() bool {
				updatedRelease := &solarv1alpha1.Release{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-stable", Namespace: namespace.Name}, updatedRelease); err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedRelease.Status.Conditions, ConditionTypeJobSucceeded)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify resources are deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-stable-renderer", Namespace: namespace.Name}, job)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())

			secret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-stable-config", Namespace: namespace.Name}, secret)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())

			// Wait and verify they are not recreated
			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-stable-renderer", Namespace: namespace.Name}, job)
				return client.IgnoreNotFound(err) == nil
			}, "2s", pollingInterval).Should(BeTrue())
		})

		It("should set JobFailed condition when job fails", func() {
			// Create a Release
			release := validRelease("test-release-failed", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-failed-renderer", Namespace: namespace.Name}, job)
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

			// Wait for Release to get JobFailed condition
			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-failed", Namespace: namespace.Name}, updatedRelease)
				if err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedRelease.Status.Conditions, ConditionTypeJobFailed)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify JobFailed condition
			condition := apimeta.FindStatusCondition(updatedRelease.Status.Conditions, ConditionTypeJobFailed)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("JobFailed"))

			// Verify job and secret still exist when failed (not cleaned up)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-failed-renderer", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-failed-config", Namespace: namespace.Name}, secret)).To(Succeed())
		})
	})

	Describe("Release deletion", func() {
		It("should cleanup resources when Release is deleted", func() {
			// Create a Release
			release := validRelease("test-release-delete", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for job and secret to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete-renderer", Namespace: namespace.Name}, job)
			}).Should(Succeed())

			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete-config", Namespace: namespace.Name}, secret)
			}).Should(Succeed())

			// Delete the Release
			createdRelease := &solarv1alpha1.Release{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete", Namespace: namespace.Name}, createdRelease)).To(Succeed())
			Expect(k8sClient.Delete(ctx, createdRelease)).To(Succeed())

			// Verify job is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete-renderer", Namespace: namespace.Name}, job)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify secret is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete-config", Namespace: namespace.Name}, secret)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify Release is deleted (finalizer removed)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-delete", Namespace: namespace.Name}, createdRelease)
			}).Should(MatchError(ContainSubstring("not found")))
		})
	})

	Describe("Release status references", func() {
		It("should maintain references to created job and secret in Release status", func() {
			// Create a Release
			release := validRelease("test-release-refs", namespace)
			Expect(k8sClient.Create(ctx, release)).To(Succeed())

			// Wait for job and secret to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-refs-renderer", Namespace: namespace.Name}, job)
			}).Should(Succeed())

			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-refs-config", Namespace: namespace.Name}, secret)
			}).Should(Succeed())

			// Verify Release status has references
			updatedRelease := &solarv1alpha1.Release{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-release-refs", Namespace: namespace.Name}, updatedRelease)
				if err != nil {
					return false
				}
				return updatedRelease.Status.JobRef != nil && updatedRelease.Status.ConfigSecretRef != nil
			}).Should(BeTrue())

			// Verify JobRef details
			Expect(updatedRelease.Status.JobRef.Name).To(Equal("test-release-refs-renderer"))
			Expect(updatedRelease.Status.JobRef.Namespace).To(Equal(namespace.Name))
			Expect(updatedRelease.Status.JobRef.Kind).To(Equal("Job"))
			Expect(updatedRelease.Status.JobRef.APIVersion).To(Equal("batch/v1"))

			// Verify ConfigSecretRef details
			Expect(updatedRelease.Status.ConfigSecretRef.Name).To(Equal("test-release-refs-config"))
			Expect(updatedRelease.Status.ConfigSecretRef.Namespace).To(Equal(namespace.Name))
			Expect(updatedRelease.Status.ConfigSecretRef.Kind).To(Equal("Secret"))
			Expect(updatedRelease.Status.ConfigSecretRef.APIVersion).To(Equal("v1"))
		})
	})
})
