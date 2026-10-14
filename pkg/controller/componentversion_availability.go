// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

func availableCondition(status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:    solarv1alpha1.ConditionTypeAvailable,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

// availabilityFromCheck maps one registry check onto the next Available condition.
// The pending state lives in the condition: the first confirmed 404 starts it,
// and only a later 404 at least grace after that start turns the condition False.
// A read error replaces pending, so the timer restarts at the next 404 and a flaky
// registry can never accumulate into a red ComponentVersion.
func availabilityFromCheck(cur *metav1.Condition, found bool, checkErr error, now time.Time, grace time.Duration) metav1.Condition {
	switch {
	case checkErr != nil:
		return availableCondition(metav1.ConditionUnknown, solarv1alpha1.ReasonRegistryUnreachable, checkErr.Error())
	case found:
		return availableCondition(metav1.ConditionTrue, solarv1alpha1.ReasonArtifactAvailable, "manifest resolves")
	case cur != nil && cur.Status == metav1.ConditionFalse:
		return *cur
	case cur != nil && cur.Reason == solarv1alpha1.ReasonArtifactMissingPending:
		if now.Sub(cur.LastTransitionTime.Time) >= grace {
			return availableCondition(metav1.ConditionFalse, solarv1alpha1.ReasonArtifactMissing, "manifest not found in registry for longer than "+grace.String())
		}

		return *cur
	default:
		c := availableCondition(metav1.ConditionUnknown, solarv1alpha1.ReasonArtifactMissingPending, "manifest not found in registry; confirming for "+grace.String())
		c.LastTransitionTime = metav1.NewTime(now)

		return c
	}
}

// setAvailableCondition stores c and reports whether anything changed. Entering
// pending from another reason restarts lastTransitionTime even though the status
// stays Unknown, because the grace period is measured from it.
func setAvailableCondition(conds *[]metav1.Condition, c metav1.Condition, now time.Time) bool {
	cur := apimeta.FindStatusCondition(*conds, solarv1alpha1.ConditionTypeAvailable)
	removed := false
	if cur != nil && c.Reason == solarv1alpha1.ReasonArtifactMissingPending && cur.Reason != solarv1alpha1.ReasonArtifactMissingPending {
		removed = apimeta.RemoveStatusCondition(conds, solarv1alpha1.ConditionTypeAvailable)
	}
	if c.LastTransitionTime.IsZero() {
		c.LastTransitionTime = metav1.NewTime(now)
	}

	return apimeta.SetStatusCondition(conds, c) || removed
}

// registryAuth returns credentials for reading from reg. Discovery and the e2e
// fixtures store username/password keys in an Opaque Secret, so those are tried
// first for any Secret type before falling back to dockerconfigjson.
func registryAuth(ctx context.Context, c client.Reader, reg *solarv1alpha1.Registry) (authn.Authenticator, error) {
	if reg.Spec.SolarSecretRef == nil {
		return authn.Anonymous, nil
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: reg.Namespace, Name: reg.Spec.SolarSecretRef.Name}, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", reg.Namespace, reg.Spec.SolarSecretRef.Name, err)
	}

	user, pass := string(secret.Data[corev1.BasicAuthUsernameKey]), string(secret.Data[corev1.BasicAuthPasswordKey])
	if user != "" || pass != "" {
		return authn.FromConfig(authn.AuthConfig{Username: user, Password: pass}), nil
	}

	return ociAuthFromSecret(secret, normalizeRegistryHost(reg.Spec.Hostname))
}
