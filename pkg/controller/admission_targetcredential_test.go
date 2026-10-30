// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	solaradmission "go.opendefense.cloud/solar/pkg/admission"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SolarTargetCredential admission", func() {
	It("only lets holders of inject-target-credential set the annotation", func() {
		role := &rbacv1.Role{
			Name:      "release-writer",
			Namespace: ns.Name,
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{solarGroup},
				Resources: []string{"releases"},
				Verbs:     []string{"create", "get"},
			}},
		}
		Expect(k8sClient.Create(ctx, role)).To(Succeed())
		Expect(k8sClient.Create(ctx, &rbacv1.RoleBinding{
			Name:      "release-writer",
			Namespace: ns.Name,
			RoleRef:   rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: role.Name},
			Subjects:  []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: "User", Name: "alice"}},
		})).To(Succeed())

		cfg := rest.CopyConfig(testEnv.GetRESTConfig())
		cfg.Impersonate = rest.ImpersonationConfig{UserName: "alice"}
		alice, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())

		newRelease := func(name string, annotations map[string]string) *solarv1alpha1.Release {
			return &solarv1alpha1.Release{
				Name:        name,
				Namespace:   ns.Name,
				Annotations: annotations,
				Spec: solarv1alpha1.ReleaseSpec{
					ComponentVersionRef: solarv1alpha1.ObjectReference{Name: "cv"},
					UniqueName:          name,
				},
			}
		}
		annotated := map[string]string{solaradmission.AnnotationTargetCredential: "true"}

		err = alice.Create(ctx, newRelease("denied", annotated))
		Expect(apierrors.IsForbidden(err)).To(BeTrue(), "got %v", err)

		Expect(alice.Create(ctx, newRelease("plain", nil))).To(Succeed())

		role.Rules[0].Verbs = append(role.Rules[0].Verbs, solaradmission.VerbInjectTargetCredential)
		Expect(k8sClient.Update(ctx, role)).To(Succeed())
		Eventually(func() error {
			return alice.Create(ctx, newRelease("allowed", annotated))
		}).Should(Succeed())
	})
})
