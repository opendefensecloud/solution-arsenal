// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Collector", func() {
	ctx := context.Background()

	Describe("CollectReleases", func() {
		listKinds := map[schema.GroupVersionResource]string{
			ociRepositoryGVR: "OCIRepositoryList",
			helmReleaseGVR:   "HelmReleaseList",
		}

		// Both halves carry a sha-truncated label (as the bootstrap chart does for
		// long names) while the annotation keeps the real Release name.
		fluxObject := func(apiVersion, kind, release, namespace string, generation int64, spec, status map[string]any) *unstructured.Unstructured {
			return &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": apiVersion,
				"kind":       kind,
				"metadata": map[string]any{
					"name":        "demo-app",
					"namespace":   namespace,
					"generation":  generation,
					"labels":      map[string]any{ReleaseLabelKey: "truncated-3f2a1b9c0d"},
					"annotations": map[string]any{ReleaseAnnotationKey: release},
				},
				"spec":   spec,
				"status": status,
			}}
		}

		ociRepositoryIn := func(namespace, release string, generation int64, status map[string]any) *unstructured.Unstructured {
			return fluxObject("source.toolkit.fluxcd.io/v1", "OCIRepository", release, namespace, generation, map[string]any{}, status)
		}

		helmReleaseIn := func(namespace, release string, generation int64, status map[string]any) *unstructured.Unstructured {
			retries := map[string]any{"remediation": map[string]any{"retries": int64(3)}}

			return fluxObject("helm.toolkit.fluxcd.io/v2", "HelmRelease", release, namespace, generation, map[string]any{
				"install": retries,
				"upgrade": retries,
			}, status)
		}

		ociRepository := func(release string, generation int64, status map[string]any) *unstructured.Unstructured {
			return ociRepositoryIn("tenant-a", release, generation, status)
		}

		helmRelease := func(release string, generation int64, status map[string]any) *unstructured.Unstructured {
			return helmReleaseIn("tenant-a", release, generation, status)
		}

		// readyStatus matches the generation the fixtures below are built with.
		readyStatus := func() map[string]any {
			return map[string]any{
				"observedGeneration": int64(1),
				"conditions": []any{
					map[string]any{"type": "Ready", "status": "True", "reason": "Succeeded"},
				},
			}
		}

		collect := func(objs ...runtime.Object) []solarv1alpha1.ReleaseReport {
			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objs...)
			c := &Collector{Dynamic: dyn, Namespace: "tenant-a"}

			releases, err := c.CollectReleases(ctx)
			Expect(err).NotTo(HaveOccurred())

			return releases
		}

		It("joins the pair, names it from the annotation and records the live chart version", func() {
			name := "a-very-long-release-name-that-the-label-cannot-hold"
			hr := helmRelease(name, 3, map[string]any{
				"observedGeneration": int64(3),
				"history":            []any{map[string]any{"chartVersion": "1.4.2"}},
				"conditions": []any{
					map[string]any{
						"type": "Ready", "status": "True",
						"reason": "InstallSucceeded", "message": "Helm install succeeded",
					},
				},
			})

			releases := collect(ociRepository(name, 1, readyStatus()), hr)
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Name).To(Equal(name))
			Expect(releases[0].Phase).To(Equal(solarv1alpha1.ReleaseReady))
			Expect(releases[0].Revision).To(Equal("1.4.2"))
			Expect(releases[0].SourceConditions).To(HaveLen(1))
			Expect(releases[0].HelmConditions).To(HaveLen(1))
			Expect(releases[0].HelmConditions[0].Reason).To(Equal("InstallSucceeded"))
		})

		It("reports Pending when only one half of the pair exists", func() {
			releases := collect(ociRepository("half-written", 1, readyStatus()))
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(solarv1alpha1.ReleasePending))
			Expect(releases[0].SourceConditions).To(HaveLen(1))
			Expect(releases[0].HelmConditions).To(BeEmpty())
		})

		It("reports Failed when the source is Stalled, not merely Degraded", func() {
			src := ociRepository("stalled-app", 2, map[string]any{
				"observedGeneration": int64(2),
				"conditions": []any{
					map[string]any{"type": "Ready", "status": "False", "reason": "OCIArtifactPullFailed"},
					map[string]any{"type": "Stalled", "status": "True", "reason": "URLInvalid"},
				},
			})

			releases := collect(src, helmRelease("stalled-app", 1, readyStatus()))
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(solarv1alpha1.ReleaseFailed))
		})

		It("reports Failed when helm remediation has run out of retries", func() {
			hr := helmRelease("exhausted-app", 2, map[string]any{
				"observedGeneration": int64(2),
				"upgradeFailures":    int64(4),
				"conditions": []any{
					map[string]any{"type": "Ready", "status": "False", "reason": "UpgradeFailed"},
					map[string]any{"type": "Remediated", "status": "True", "reason": "RollbackSucceeded"},
				},
			})

			releases := collect(ociRepository("exhausted-app", 1, readyStatus()), hr)
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(solarv1alpha1.ReleaseFailed))
		})

		It("reports Degraded while retries remain", func() {
			hr := helmRelease("retrying-app", 2, map[string]any{
				"observedGeneration": int64(2),
				"upgradeFailures":    int64(1),
				"conditions": []any{
					map[string]any{"type": "Ready", "status": "False", "reason": "UpgradeFailed"},
					map[string]any{"type": "Remediated", "status": "True", "reason": "RollbackSucceeded"},
				},
			})

			releases := collect(ociRepository("retrying-app", 1, readyStatus()), hr)
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(solarv1alpha1.ReleaseDegraded))
		})

		// Same Release name, two namespaces: keying on the name alone would drop one
		// pair and cross-pair the other's halves.
		It("keeps same-named releases in different namespaces apart", func() {
			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds,
				ociRepositoryIn("tenant-a", "shared-name", 1, readyStatus()),
				helmReleaseIn("tenant-a", "shared-name", 1, readyStatus()),
				ociRepositoryIn("tenant-b", "shared-name", 1, readyStatus()),
			)
			c := &Collector{Dynamic: dyn, Namespace: ""}

			releases, err := c.CollectReleases(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(releases).To(HaveLen(2))

			Expect(releases[0].Namespace).To(Equal("tenant-a"))
			Expect(releases[0].Phase).To(Equal(solarv1alpha1.ReleaseReady))

			// tenant-b has no HelmRelease of its own and must not borrow tenant-a's.
			Expect(releases[1].Namespace).To(Equal("tenant-b"))
			Expect(releases[1].Phase).To(Equal(solarv1alpha1.ReleasePending))
		})

		// Exactly what a failing install looks like on the dev cluster: Flux keeps
		// Reconciling=True while it retries, so Reconciling must not mask Ready=False.
		// helm-controller parks status.observedGeneration at -1 for the whole time a
		// reconciliation is in flight. Gating on it would hide the failure below.
		It("trusts current conditions even while status.observedGeneration is -1", func() {
			hr := helmRelease("in-flight", 1, map[string]any{
				"observedGeneration": int64(-1),
				"conditions": []any{
					map[string]any{
						"type": "Reconciling", "status": "True",
						"reason": "Progressing", "observedGeneration": int64(1),
					},
					map[string]any{
						"type": "Ready", "status": "False",
						"reason": "InstallFailed", "observedGeneration": int64(1),
					},
				},
			})

			releases := collect(ociRepository("in-flight", 1, readyStatus()), hr)
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(solarv1alpha1.ReleaseDegraded))
		})

		It("reports Degraded, not Progressing, while Flux retries a failed install", func() {
			hr := helmRelease("failing-install", 1, map[string]any{
				"observedGeneration": int64(1),
				"conditions": []any{
					map[string]any{"type": "Reconciling", "status": "True", "reason": "Progressing"},
					map[string]any{"type": "Ready", "status": "False", "reason": "InstallFailed"},
					map[string]any{"type": "Released", "status": "False", "reason": "InstallFailed"},
				},
			})

			releases := collect(ociRepository("failing-install", 1, readyStatus()), hr)
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(solarv1alpha1.ReleaseDegraded))
		})

		It("reports Progressing when Ready=True describes a stale generation", func() {
			hr := helmRelease("stale-app", 5, map[string]any{
				"observedGeneration": int64(4),
				"conditions": []any{
					// stamped with the generation it describes, one behind the spec
					map[string]any{
						"type": "Ready", "status": "True",
						"reason": "UpgradeSucceeded", "observedGeneration": int64(4),
					},
				},
			})

			releases := collect(ociRepository("stale-app", 1, readyStatus()), hr)
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(solarv1alpha1.ReleaseProgressing))
		})

		It("reports Degraded when the source is Ready but the HelmRelease is not", func() {
			hr := helmRelease("broken-app", 2, map[string]any{
				"observedGeneration": int64(2),
				"conditions": []any{
					map[string]any{"type": "Ready", "status": "False", "reason": "InstallFailed"},
				},
			})

			releases := collect(ociRepository("broken-app", 1, readyStatus()), hr)
			Expect(releases).To(HaveLen(1))
			Expect(releases[0].Phase).To(Equal(solarv1alpha1.ReleaseDegraded))
		})
	})
})
