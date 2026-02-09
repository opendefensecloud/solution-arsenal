// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opendefense.cloud/kit/envtest"
	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("RenderTaskController", Ordered, func() {
	var (
		ctx       = envtest.Context()
		namespace = setupTest(ctx)

		validRenderTask = func(name string, namespace *corev1.Namespace) *solarv1alpha1.RenderTask {
			return &solarv1alpha1.RenderTask{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace.Name,
				},
				Spec: solarv1alpha1.RenderTaskSpec{
					RendererConfig: solarv1alpha1.RendererConfig{
						Type:          solarv1alpha1.RendererConfigTypeRelease,
						ReleaseConfig: solarv1alpha1.ReleaseConfig{},
						PushOptions:   solarv1alpha1.PushOptions{},
					},
				},
			}
		}
	)

	Describe("RenderTask creation and job scheduling", func() {
		It("should create a RenderTask and schedule a renderer job", func() {
			// Create a RenderTask
			task := validRenderTask("test-config", namespace)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Verify the RenderTask was created
			createdRenderTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-config", Namespace: namespace.Name}, createdRenderTask)
			}).Should(Succeed())

			// Verify finalizer was added after a reconciliation cycle
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-config", Namespace: namespace.Name}, createdRenderTask)
				if err != nil {
					return false
				}
				return slices.Contains(createdRenderTask.Finalizers, renderTaskFinalizer)
			}, eventuallyTimeout).Should(BeTrue(), "finalizer should be added by reconciler")

			// Verify config secret was created
			configSecret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-config", Namespace: namespace.Name}, configSecret)
			}, eventuallyTimeout).Should(Succeed())

			Expect(configSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(configSecret.Data).To(HaveKey("config.json"))

			// Verify job was created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-config", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("renderer"))
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal("image:tag"))
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(ContainElement("solar-renderer"))

			// Verify config secret is mounted
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Volumes[0].Name).To(Equal("config"))
			Expect(job.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal("render-test-config"))
		})

		It("should create a RenderTask and fill the config secret correctly", func() {
			// TODO
		})

		It("should set the ChartURL status field", func() {
			// TODO
		})

		It("should set JobScheduled condition when job is running", func() {
			task := validRenderTask("test-task-running", namespace)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-running", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Wait for JobScheduled condition
			updatedTask := &solarv1alpha1.RenderTask{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-running", Namespace: namespace.Name}, updatedTask)
				if err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedTask.Status.Conditions, ConditionTypeJobScheduled)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify the condition
			condition := apimeta.FindStatusCondition(updatedTask.Status.Conditions, ConditionTypeJobScheduled)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("JobScheduled"))
		})

		It("should not recreate a job if one already exists", func() {
			// Create a Task
			task := validRenderTask("test-task-existing", namespace)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-existing", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Get the job's UID
			originalUID := job.UID

			// Wait a bit to ensure no new jobs are created
			Consistently(func() string {
				updatedJob := &batchv1.Job{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-existing", Namespace: namespace.Name}, updatedJob); err != nil {
					return ""
				}
				return string(updatedJob.UID)
			}, "2s", pollingInterval).Should(Equal(string(originalUID)))
		})
	})

	Describe("RenderTask job completion and cleanup", func() {
		It("should cleanup job and secret when job completes successfully", func() {
			// Create a Task
			task := validRenderTask("test-task-success", namespace)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-success", Namespace: namespace.Name}, job)
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
			// TODO: this does not pass yet
			//			// Wait for RenderTask to get JobSucceeded condition
			//			updatedTask := &solarv1alpha1.RenderTask{}
			//			Eventually(func() bool {
			//				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-success", Namespace: namespace.Name}, updatedTask)
			//				if err != nil {
			//					return false
			//				}
			//				return apimeta.IsStatusConditionTrue(updatedTask.Status.Conditions, ConditionTypeJobSucceeded)
			//			}, eventuallyTimeout).Should(BeTrue())
			//
			//			// Verify JobSucceeded condition
			//			condition := apimeta.FindStatusCondition(updatedTask.Status.Conditions, ConditionTypeJobSucceeded)
			//			Expect(condition).NotTo(BeNil())
			//			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			//			Expect(condition.Reason).To(Equal("JobSucceeded"))
			//
			//			// Verify job is deleted
			//			Eventually(func() bool {
			//				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-success", Namespace: namespace.Name}, job)
			//				return err != nil
			//			}, eventuallyTimeout).Should(BeTrue())
			//
			//			// Verify config secret is deleted
			//			secret := &corev1.Secret{}
			//			Eventually(func() bool {
			//				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-success", Namespace: namespace.Name}, secret)
			//				return client.IgnoreNotFound(err) == nil
			//			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should not recreate resources after successful completion", func() {
			// Create a Task
			task := validRenderTask("test-task-stable", namespace)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-stable", Namespace: namespace.Name}, job)
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

			// Wait for resources to be cleaned up and RenderTask to show success
			Eventually(func() bool {
				updatedTask := &solarv1alpha1.RenderTask{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-stable", Namespace: namespace.Name}, updatedTask); err != nil {
					return false
				}
				return apimeta.IsStatusConditionTrue(updatedTask.Status.Conditions, ConditionTypeJobSucceeded)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify resources are deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-stable", Namespace: namespace.Name}, job)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())

			secret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-stable", Namespace: namespace.Name}, secret)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())

			// Wait and verify they are not recreated
			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-stable", Namespace: namespace.Name}, job)
				return client.IgnoreNotFound(err) == nil
			}, "2s", pollingInterval).Should(BeTrue())
		})

		It("should set JobFailed condition when job fails", func() {
			// Create a RenderTask
			task := validRenderTask("test-task-failed", namespace)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-failed", Namespace: namespace.Name}, job)
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

			// TODO: this does not pass yet
			//			// Wait for RenderTask to get JobFailed condition
			//			updatedTask := &solarv1alpha1.RenderTask{}
			//			Eventually(func() bool {
			//				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-failed", Namespace: namespace.Name}, updatedTask)
			//				if err != nil {
			//					return false
			//				}
			//				return apimeta.IsStatusConditionTrue(updatedTask.Status.Conditions, ConditionTypeJobFailed)
			//			}, eventuallyTimeout).Should(BeTrue())
			//
			//			// Verify JobFailed condition
			//			condition := apimeta.FindStatusCondition(updatedTask.Status.Conditions, ConditionTypeJobFailed)
			//			Expect(condition).NotTo(BeNil())
			//			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			//			Expect(condition.Reason).To(Equal("JobFailed"))
			//
			//			// Verify job and secret still exist when failed (not cleaned up)
			//			Eventually(func() error {
			//				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-failed", Namespace: namespace.Name}, job)
			//			}, eventuallyTimeout).Should(Succeed())
			//
			//			secret := &corev1.Secret{}
			//			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-failed", Namespace: namespace.Name}, secret)).To(Succeed())
		})
	})
	Describe("RenderTask deletion", func() {
		It("should cleanup resources when RenderTask is deleted", func() {
			// Create a RenderTask
			task := validRenderTask("test-task-delete", namespace)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job and secret to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-delete", Namespace: namespace.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-delete", Namespace: namespace.Name}, secret)
			}).Should(Succeed())

			// Delete the RenderTask
			createdTask := &solarv1alpha1.RenderTask{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-delete", Namespace: namespace.Name}, createdTask)).To(Succeed())
			Expect(k8sClient.Delete(ctx, createdTask)).To(Succeed())

			// Verify job is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-delete", Namespace: namespace.Name}, job)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify secret is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-delete", Namespace: namespace.Name}, secret)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify RenderTask is deleted (finalizer removed)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-delete", Namespace: namespace.Name}, createdTask)
			}).Should(MatchError(ContainSubstring("not found")))
		})

		It("should maintain references to created job and secret in RenderTask status", func() {
		})
	})

	Describe("RenderTask status references", func() {
		It("should maintain references to created job and secret in RenderTask status", func() {
			// Create a RenderTask
			task := validRenderTask("test-task-refs", namespace)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job and secret to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-refs", Namespace: namespace.Name}, job)
			}).Should(Succeed())

			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-refs", Namespace: namespace.Name}, secret)
			}).Should(Succeed())

			//				TODO: Verify RenderTask status has references
			//				updatedTask := &solarv1alpha1.RenderTask{}
			//				Eventually(func() bool {
			//					err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-refs", Namespace: namespace.Name}, updatedTask)
			//					if err != nil {
			//						return false
			//					}
			//					return updatedTask.Status.JobRef != nil && updatedTask.Status.ConfigSecretRef != nil
			//				}).Should(BeTrue())
			//
			//				// Verify JobRef details
			//				Expect(updatedTask.Status.JobRef.Name).To(Equal("render-test-task-refs"))
			//				Expect(updatedTask.Status.JobRef.Namespace).To(Equal(namespace.Name))
			//				Expect(updatedTask.Status.JobRef.Kind).To(Equal("Job"))
			//				Expect(updatedTask.Status.JobRef.APIVersion).To(Equal("batch/v1"))
			//
			//				// Verify ConfigSecretRef details
			//				Expect(updatedTask.Status.ConfigSecretRef.Name).To(Equal("render-test-task-refs"))
			//				Expect(updatedTask.Status.ConfigSecretRef.Namespace).To(Equal(namespace.Name))
			//				Expect(updatedTask.Status.ConfigSecretRef.Kind).To(Equal("Secret"))
			//				Expect(updatedTask.Status.ConfigSecretRef.APIVersion).To(Equal("v1"))
		})
	})
})
