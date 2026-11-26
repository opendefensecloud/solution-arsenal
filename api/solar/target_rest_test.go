// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar_test

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"go.opendefense.cloud/solar/api/solar"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Target REST", func() {
	signed := func(privateKeySecret, publicKeySecret string) *solar.Target {
		return &solar.Target{Spec: solar.TargetSpec{Signing: &solar.TargetSigning{
			SolarPrivateKeySecretRef:  corev1.LocalObjectReference{Name: privateKeySecret},
			TargetPublicKeySecretName: publicKeySecret,
		}}}
	}

	fields := func(t *solar.Target) []string {
		var out []string
		for _, err := range t.Validate(context.Background()) {
			out = append(out, err.Field)
		}

		return out
	}

	It("accepts an unsigned Target", func() {
		Expect((&solar.Target{}).Validate(context.Background())).To(BeEmpty())
	})

	It("accepts signing with both Secrets named", func() {
		Expect(signed("key", "pub").Validate(context.Background())).To(BeEmpty())
	})

	It("rejects signing without a public key Secret", func() {
		Expect(fields(signed("key", ""))).To(Equal([]string{"spec.signing.targetPublicKeySecretName"}))
	})

	It("rejects signing without a signing key Secret", func() {
		Expect(fields(signed("", "pub"))).To(Equal([]string{"spec.signing.solarPrivateKeySecretRef.name"}))
	})

	It("rejects a public key Secret name that is not a valid object name", func() {
		errs := signed("key", "Not_Valid").Validate(context.Background())
		Expect(errs).NotTo(BeEmpty())
		Expect(errs[0].Field).To(Equal("spec.signing.targetPublicKeySecretName"))
		Expect(errs[0].Type).To(Equal(field.ErrorTypeInvalid))
	})

	It("rejects a signing key Secret name that is not a valid object name", func() {
		Expect(fields(signed("Not_Valid", "pub"))).To(Equal([]string{"spec.signing.solarPrivateKeySecretRef.name"}))
	})

	It("applies the same rules on update", func() {
		old := signed("key", "pub")
		updated := signed("key", "")
		errs := updated.ValidateUpdate(context.Background(), old)
		Expect(errs).To(HaveLen(1))
		Expect(errs[0].Field).To(Equal("spec.signing.targetPublicKeySecretName"))
	})
})
