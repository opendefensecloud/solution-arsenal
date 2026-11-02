// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/client-go/clientset/versioned/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KubePublisher", func() {
	report := func() solarv1alpha1.TargetReport {
		return solarv1alpha1.TargetReport{Name: "edge-01", Namespace: "ns",
			Spec: solarv1alpha1.TargetReportSpec{LastReportTime: metav1.Now()}}
	}

	It("does not create a missing report: the controller pre-creates it", func() {
		cs := fake.NewSimpleClientset()
		err := KubePublisher{Client: cs}.Publish(context.Background(), report())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		list, _ := cs.SolarV1alpha1().TargetReports("ns").List(context.Background(), metav1.ListOptions{})
		Expect(list.Items).To(BeEmpty())
	})

	It("updates an existing report", func() {
		existing := report()
		cs := fake.NewSimpleClientset(&existing)
		Expect(KubePublisher{Client: cs}.Publish(context.Background(), report())).To(Succeed())
	})

	It("keeps the stored ownerReferences and labels and replaces the spec", func() {
		seeded := solarv1alpha1.TargetReport{Name: "edge-01", Namespace: "ns",
			Labels: map[string]string{"keep": "me"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "solar.opendefense.cloud/v1alpha1", Kind: "Target", Name: "edge-01", UID: "uid-1", Controller: new(true),
			}}}
		cs := fake.NewSimpleClientset(&seeded)
		published := report()
		Expect(KubePublisher{Client: cs}.Publish(context.Background(), published)).To(Succeed())

		got, err := cs.SolarV1alpha1().TargetReports("ns").Get(context.Background(), "edge-01", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(got.OwnerReferences).To(Equal(seeded.OwnerReferences))
		Expect(got.Labels).To(HaveKeyWithValue("keep", "me"))
		Expect(got.Spec.LastReportTime.Time).To(BeTemporally("~", published.Spec.LastReportTime.Time, time.Second))
	})
})
