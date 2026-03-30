// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RenderTaskController", Ordered, func() {
	var (
		validRenderTask = func(name string, _ *corev1.Namespace) *solarv1alpha1.RenderTask {
			return &solarv1alpha1.RenderTask{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: solarv1alpha1.RenderTaskSpec{
					RendererConfig: solarv1alpha1.RendererConfig{
						Type: solarv1alpha1.RendererConfigTypeRelease,
						ReleaseConfig: solarv1alpha1.ReleaseConfig{
							Chart: solarv1alpha1.ChartConfig{
								Name:        "my-release",
								Description: "release for my componentversion",
								Version:     "v1.0.0",
								AppVersion:  "v1.0.0",
							},
							Input: solarv1alpha1.ReleaseInput{
								Component: solarv1alpha1.ReleaseComponent{
									Name: "my-component",
								},
								Resources: map[string]solarv1alpha1.ResourceAccess{
									"foo": {Repository: "example.com", Tag: "v1.0.0"},
								},
								Entrypoint: solarv1alpha1.Entrypoint{
									ResourceName: "foo",
									Type:         solarv1alpha1.EntrypointTypeHelm,
								},
							},
						},
					},
					Repository: "my-release",
					Tag:        "v1.0.0",
				},
			}
		}

		simulateJobCompletion = func(job *batchv1.Job) error {
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

			return k8sClient.Status().Update(ctx, job)
		}
	)

	Describe("RenderTask creation and job scheduling", func() {
		It("should set TTLSecondsAfterFinished on job from FailedJobTTL", func() {
			task := validRenderTask("test-task-ttl", ns)
			ttl := int32(7200)
			task.Spec.FailedJobTTL = &ttl
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-ttl", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			Expect(job.Spec.TTLSecondsAfterFinished).ToNot(BeNil())
			Expect(*job.Spec.TTLSecondsAfterFinished).To(Equal(int32(7200)))
		})

		It("should use default TTLSecondsAfterFinished when FailedJobTTL is not set", func() {
			task := validRenderTask("test-task-no-ttl", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-no-ttl", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			Expect(job.Spec.TTLSecondsAfterFinished).ToNot(BeNil())
			Expect(*job.Spec.TTLSecondsAfterFinished).To(Equal(int32(3600)))
		})

		It("should create a RenderTask and schedule a renderer job", func() {
			// Create a RenderTask
			task := validRenderTask("test-config", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Verify the RenderTask was created
			createdRenderTask := &solarv1alpha1.RenderTask{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-config"}, createdRenderTask)
			}).Should(Succeed())

			// Verify finalizer was added after a reconciliation cycle
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-config"}, createdRenderTask)
				if err != nil {
					return false
				}

				return slices.Contains(createdRenderTask.Finalizers, renderTaskFinalizer)
			}, eventuallyTimeout).Should(BeTrue(), "finalizer should be added by reconciler")

			// Verify config secret was created
			configSecret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-config", Namespace: ns.Name}, configSecret)
			}, eventuallyTimeout).Should(Succeed())

			Expect(configSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(configSecret.Data).To(HaveKey("config.json"))

			// Verify job was created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-config", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Name).To(Equal("renderer"))
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal("image:tag"))
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(ContainElement("solar-renderer"))

			// Verify config secret is mounted (plus ca-bundle when RendererCAConfigMap is set)
			Expect(job.Spec.Template.Spec.Volumes).To(HaveLen(2))
			Expect(job.Spec.Template.Spec.Volumes[0].Name).To(Equal("config"))
			Expect(job.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal("render-test-config"))
			Expect(job.Spec.Template.Spec.Volumes[1].Name).To(Equal("ca-bundle"))
			Expect(job.Spec.Template.Spec.Volumes[1].ConfigMap.Name).To(Equal("root-bundle"))

			// Verify volume mounts
			container := job.Spec.Template.Spec.Containers[0]
			Expect(container.VolumeMounts).To(HaveLen(2))
			Expect(container.VolumeMounts[0].Name).To(Equal("config"))
			Expect(container.VolumeMounts[1].Name).To(Equal("ca-bundle"))

			// Verify SSL_CERT_FILE env var
			Expect(container.Env).To(ContainElement(corev1.EnvVar{
				Name:  "SSL_CERT_FILE",
				Value: "/etc/ssl/certs/ca-bundle.pem",
			}))
		})

		It("should create a RenderTask and fill the config secret correctly", func() {
			// Create a RenderTask
			task := validRenderTask("test-config-content", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Verify config secret was created
			configSecret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-config-content", Namespace: ns.Name}, configSecret)
			}, eventuallyTimeout).Should(Succeed())

			Expect(configSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(configSecret.Data).To(HaveKey("config.json"))
			Expect(configSecret.Data["config.json"]).NotTo(BeNil())

			cfg := &solarv1alpha1.RendererConfig{}

			Expect(json.Unmarshal(configSecret.Data["config.json"], cfg)).To(Succeed())

			Expect(cfg.Type).To(Equal(solarv1alpha1.RendererConfigTypeRelease))
			Expect(cfg.ReleaseConfig.Input.Resources).NotTo(BeNil())
			Expect(cfg.ReleaseConfig.Input.Resources["foo"].Repository).To(Equal("example.com"))
			Expect(cfg.ReleaseConfig.Chart.Version).To(Equal("v1.0.0"))
		})

		It("should set the ChartURL status field", func() {
			// Create a RenderTask
			task := validRenderTask("test-task-url", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-url", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			Expect(simulateJobCompletion(job)).To(Succeed())

			// Wait for ChartURL to be in Status
			createdTask := &solarv1alpha1.RenderTask{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-url"}, createdTask); err != nil {
					return false
				}

				return createdTask.Status.ChartURL != ""
			}).Should(BeTrue())

			Expect(createdTask.Status.ChartURL).To(Equal("oci://example.com/my-release:v1.0.0"))
		})

		It("should set JobScheduled condition when job is running", func() {
			task := validRenderTask("test-task-running", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-running", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Wait for JobScheduled condition
			updatedTask := &solarv1alpha1.RenderTask{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-running"}, updatedTask)
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
			task := validRenderTask("test-task-existing", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-existing", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Get the job's UID
			originalUID := job.UID

			// Wait a bit to ensure no new jobs are created
			Consistently(func() string {
				updatedJob := &batchv1.Job{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-existing", Namespace: ns.Name}, updatedJob); err != nil {
					return ""
				}

				return string(updatedJob.UID)
			}, "2s", pollingInterval).Should(Equal(string(originalUID)))
		})
	})

	Describe("RenderTask job completion and cleanup", func() {
		It("should cleanup job and secret when job completes successfully", func() {
			// Create a Task
			task := validRenderTask("test-task-success", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-success", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Simulate job completion by updating its status
			Expect(simulateJobCompletion(job)).To(Succeed())

			// Wait for RenderTask to get JobSucceeded condition
			updatedTask := &solarv1alpha1.RenderTask{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-success"}, updatedTask)
				if err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(updatedTask.Status.Conditions, ConditionTypeJobSucceeded)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify JobSucceeded condition
			condition := apimeta.FindStatusCondition(updatedTask.Status.Conditions, ConditionTypeJobSucceeded)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("JobSucceeded"))

			// Verify job is deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-success", Namespace: ns.Name}, job)
				return err != nil
			}, eventuallyTimeout).Should(BeTrue())

			// Verify config secret is deleted
			secret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-success", Namespace: ns.Name}, secret)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())
		})

		It("should not recreate resources after successful completion", func() {
			// Create a Task
			task := validRenderTask("test-task-stable", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-stable", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			Expect(simulateJobCompletion(job)).To(Succeed())

			// Wait for resources to be cleaned up and RenderTask to show success
			Eventually(func() bool {
				updatedTask := &solarv1alpha1.RenderTask{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-stable"}, updatedTask); err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(updatedTask.Status.Conditions, ConditionTypeJobSucceeded)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify resources are deleted
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-stable", Namespace: ns.Name}, job)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())

			secret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-stable", Namespace: ns.Name}, secret)
				return client.IgnoreNotFound(err) == nil
			}, eventuallyTimeout).Should(BeTrue())

			// Wait and verify they are not recreated
			Consistently(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-stable", Namespace: ns.Name}, job)
				return client.IgnoreNotFound(err) == nil
			}, "2s", pollingInterval).Should(BeTrue())
		})

		It("should set JobFailed condition when job fails", func() {
			// Create a RenderTask
			task := validRenderTask("test-task-failed", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-failed", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Simulate job failure by updating its status
			job.Status.Failed = 1
			now := metav1.Now()
			job.Status.StartTime = &now

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

			// Wait for RenderTask to get JobFailed condition
			updatedTask := &solarv1alpha1.RenderTask{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-failed"}, updatedTask); err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(updatedTask.Status.Conditions, ConditionTypeJobFailed)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify JobFailed condition
			condition := apimeta.FindStatusCondition(updatedTask.Status.Conditions, ConditionTypeJobFailed)
			Expect(condition).NotTo(BeNil())
			Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			Expect(condition.Reason).To(Equal("JobFailed"))

			// Verify job and secret still exist when failed (not cleaned up before TTL)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-failed", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-failed", Namespace: ns.Name}, secret)
			}, eventuallyTimeout).Should(Succeed())
		})

		It("should cleanup secrets after FailedJobTTL on job failure", func() {
			// Create a RenderTask with short TTL for testing
			task := validRenderTask("test-task-failed-ttl", ns)
			ttl := int32(2)
			task.Spec.FailedJobTTL = &ttl
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-failed-ttl", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			// Simulate job failure (CompletionTime cannot be set for failed jobs)
			now := metav1.Now()
			job.Status.Failed = 1
			job.Status.StartTime = &now
			job.Status.Conditions = []batchv1.JobCondition{
				{
					Type:               batchv1.JobFailureTarget,
					Status:             corev1.ConditionTrue,
					LastProbeTime:      now,
					LastTransitionTime: now,
				},
				{
					Type:               batchv1.JobFailed,
					Status:             corev1.ConditionTrue,
					LastProbeTime:      now,
					LastTransitionTime: now,
				},
			}
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())

			// Wait for JobFailed condition
			updatedTask := &solarv1alpha1.RenderTask{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-failed-ttl"}, updatedTask); err != nil {
					return false
				}

				return apimeta.IsStatusConditionTrue(updatedTask.Status.Conditions, ConditionTypeJobFailed)
			}, eventuallyTimeout).Should(BeTrue())

			// Verify secret exists before TTL expires
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-failed-ttl", Namespace: ns.Name}, secret)).To(Succeed())

			// Wait for TTL to expire
			time.Sleep(3 * time.Second)

			// Verify secret is deleted after TTL
			Eventually(func() bool {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-failed-ttl", Namespace: ns.Name}, secret) != nil
			}, eventuallyTimeout).Should(BeTrue())
		})
	})
	Describe("RenderTask deletion", func() {
		It("should cleanup resources when RenderTask is deleted", func() {
			// Create a RenderTask
			task := validRenderTask("test-task-delete", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job and secret to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-delete", Namespace: ns.Name}, job)
			}, eventuallyTimeout).Should(Succeed())

			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-delete", Namespace: ns.Name}, secret)
			}).Should(Succeed())

			// Delete the RenderTask
			createdTask := &solarv1alpha1.RenderTask{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-delete"}, createdTask)).To(Succeed())
			Expect(k8sClient.Delete(ctx, createdTask)).To(Succeed())

			// Verify job is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-delete", Namespace: ns.Name}, job)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify secret is deleted
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-delete", Namespace: ns.Name}, secret)
			}).Should(MatchError(ContainSubstring("not found")))

			// Verify RenderTask is deleted (finalizer removed)
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-delete"}, createdTask)
			}).Should(MatchError(ContainSubstring("not found")))
		})

		It("should maintain references to created job and secret in RenderTask status", Pending, func() {
		})
	})

	Describe("RenderTask status references", func() {
		It("should maintain references to created job and secret in RenderTask status", func() {
			// Create a RenderTask
			task := validRenderTask("test-task-refs", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job and secret to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-refs", Namespace: ns.Name}, job)
			}).Should(Succeed())

			secret := &corev1.Secret{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-refs", Namespace: ns.Name}, secret)
			}).Should(Succeed())

			// Verify RenderTask status has references
			updatedTask := &solarv1alpha1.RenderTask{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-refs"}, updatedTask)
				if err != nil {
					return false
				}

				return updatedTask.Status.JobRef != nil && updatedTask.Status.ConfigSecretRef != nil
			}).Should(BeTrue())

			// Verify JobRef details
			Expect(updatedTask.Status.JobRef.Name).To(Equal("render-test-task-refs"))
			Expect(updatedTask.Status.JobRef.Namespace).To(Equal(ns.Name))
			Expect(updatedTask.Status.JobRef.Kind).To(Equal("Job"))
			Expect(updatedTask.Status.JobRef.APIVersion).To(Equal("batch/v1"))

			// Verify ConfigSecretRef details
			Expect(updatedTask.Status.ConfigSecretRef.Name).To(Equal("render-test-task-refs"))
			Expect(updatedTask.Status.ConfigSecretRef.Namespace).To(Equal(ns.Name))
			Expect(updatedTask.Status.ConfigSecretRef.Kind).To(Equal("Secret"))
			Expect(updatedTask.Status.ConfigSecretRef.APIVersion).To(Equal("v1"))
		})
	})
	Describe("RenderTask immutablility", func() {
		It("should not allow to changes to RendererConfig", func() {
			// Create a RenderTask
			task := validRenderTask("test-task-update", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			Consistently(func() error {
				latest := &solarv1alpha1.RenderTask{}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-task-update"}, latest); err != nil {
					return err
				}
				latest.Spec.RendererConfig.Type = solarv1alpha1.RendererConfigTypeProfile

				return k8sClient.Update(ctx, latest)
			}, "5s", pollingInterval).ShouldNot(Succeed())
		})
	})
	Describe("RenderTask Secret References", func() {
		It("should pass credentials to job when basic-auth secret is configured", func() {
			// replace dummy secret with basic-auth
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rendertask-secret",
					Namespace: "default",
				},
			}
			Expect(k8sClient.Delete(ctx, secret.DeepCopy())).To(Succeed())

			secret.Type = corev1.SecretTypeBasicAuth
			secret.StringData = map[string]string{
				"username": "foo",
				"password": "bar",
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			// Create a RenderTask
			task := validRenderTask("test-task-basicauth", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-basicauth", Namespace: ns.Name}, job)
			}).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name: "REGISTRY_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "rendertask-secret",
						},
						Key: "username",
					},
				},
			}))
			Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name: "REGISTRY_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "rendertask-secret",
						},
						Key: "password",
					},
				},
			}))
		})

		It("should pass dockerconfig to job when dockerconfig secret is configured", func() {
			// replace dummy secret with dockerconfig
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rendertask-secret",
					Namespace: "default",
				},
			}
			Expect(k8sClient.Delete(ctx, secret.DeepCopy())).To(Succeed())

			secret.Type = corev1.SecretTypeDockerConfigJson
			auth := base64.StdEncoding.EncodeToString([]byte("foo:bar"))
			secret.StringData = map[string]string{
				".dockerconfigjson": fmt.Sprintf(`{"auths":{"example.com":{"auth":"%s"}}}`, auth),
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			// Create a RenderTask
			task := validRenderTask("test-task-dockerconfig", ns)
			Expect(k8sClient.Create(ctx, task)).To(Succeed())

			// Wait for job to be created
			job := &batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Name: "render-test-task-dockerconfig", Namespace: ns.Name}, job)
			}).Should(Succeed())

			Expect(job.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
				Name: "dockerconfig",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "rendertask-secret",
						Items: []corev1.KeyToPath{
							{
								Key:  ".dockerconfigjson",
								Path: "dockerconfig.json",
							},
						},
						DefaultMode: ptr.To[int32](0o644),
					},
				},
			}))

			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{
				Name:      "dockerconfig",
				ReadOnly:  true,
				MountPath: "/etc/renderer/dockerconfig.json",
				SubPath:   "dockerconfig.json",
			}))

			Expect(job.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "DOCKER_CONFIG",
				Value: "/etc/renderer/dockerconfig.json",
			}))
		})
	})
})
