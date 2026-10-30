// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const (
	// LabelAgentCredential marks a Target's agent credential Secrets; the value is the Target name.
	LabelAgentCredential = "solar.opendefense.cloud/agent-credential"
	// AnnotationAgentCredential set to AgentCredentialRevoked on a Target revokes its agent credential.
	AnnotationAgentCredential = "solar.opendefense.cloud/agent-credential"
	// AnnotationRotateAgentCredential on a Target requests a new credential whenever its value changes.
	AnnotationRotateAgentCredential = "solar.opendefense.cloud/rotate-agent-credential"
	// AnnotationRotationRequest on a credential Secret records the rotate value it answered.
	AnnotationRotationRequest = "solar.opendefense.cloud/rotation-request"
	// AnnotationExpiresAt on a credential Secret is the token's expiry, RFC 3339.
	AnnotationExpiresAt = "solar.opendefense.cloud/expires-at"

	AgentCredentialRevoked  = "revoked"
	AgentCredentialTokenKey = "token"

	ConditionTypeAgentCredentialReady = "AgentCredentialReady"
	ReasonAgentCredentialDisabled     = "Disabled"
	ReasonAgentCredentialRevoked      = "Revoked"
	ReasonAgentCredentialMintFailed   = "MintFailed"
	ReasonAgentCredentialIssued       = "Issued"
)

func agentName(target string) string { return "solar-agent-" + target }

func credentialSecretName(target string, n int) string {
	return fmt.Sprintf("%s-%d", agentName(target), n)
}

// agentRules is everything an agent may do: read its Target, read and update
// its pre-created report. RBAC cannot scope create to one name, hence no create.
func agentRules(target string) []rbacv1.PolicyRule {
	group := solarv1alpha1.SchemeGroupVersion.Group

	return []rbacv1.PolicyRule{
		{APIGroups: []string{group}, Resources: []string{"targets"}, ResourceNames: []string{target}, Verbs: []string{"get"}},
		{APIGroups: []string{group}, Resources: []string{"targetreports"}, ResourceNames: []string{target}, Verbs: []string{"get", "update"}},
	}
}

// credentialIndex returns n for a Secret named solar-agent-<target>-<n>, n ≥ 1.
func credentialIndex(target string, s *corev1.Secret) (int, bool) {
	suffix, found := strings.CutPrefix(s.Name, agentName(target)+"-")
	if !found {
		return 0, false
	}
	n, err := strconv.Atoi(suffix)
	if err != nil || n < 1 {
		return 0, false
	}

	return n, true
}

// listAgentCredentials returns t's credential Secrets, oldest first.
func listAgentCredentials(ctx context.Context, c client.Reader, t *solarv1alpha1.Target) ([]corev1.Secret, error) {
	list := &corev1.SecretList{}
	if err := c.List(ctx, list, client.InNamespace(t.Namespace), client.MatchingLabels{LabelAgentCredential: t.Name}); err != nil {
		return nil, fmt.Errorf("listing agent credentials: %w", err)
	}

	out := slices.DeleteFunc(list.Items, func(s corev1.Secret) bool { _, ok := credentialIndex(t.Name, &s); return !ok })
	slices.SortFunc(out, func(a, b corev1.Secret) int {
		ia, _ := credentialIndex(t.Name, &a)
		ib, _ := credentialIndex(t.Name, &b)

		return ia - ib
	})

	return out, nil
}

// currentAgentCredential is the Secret to render for t: the newest one that
// already holds a token. Nil when t is revoked or has none yet.
func currentAgentCredential(ctx context.Context, c client.Reader, t *solarv1alpha1.Target) (*corev1.Secret, error) {
	if t.Annotations[AnnotationAgentCredential] == AgentCredentialRevoked {
		return nil, nil //nolint:nilnil // nil means "no credential"
	}
	secrets, err := listAgentCredentials(ctx, c, t)
	if err != nil {
		return nil, err
	}
	for i := len(secrets) - 1; i >= 0; i-- {
		if len(secrets[i].Data[AgentCredentialTokenKey]) > 0 {
			return &secrets[i], nil
		}
	}

	return nil, nil //nolint:nilnil // nil means "no credential"
}

// credentialTag identifies a credential in chart tags. It hashes the UID, not
// the name: names repeat after a revoke, and a repeated tag would make the
// renderer skip the push and ship the old, revoked token.
func credentialTag(s *corev1.Secret) string {
	sum := sha256.Sum256([]byte(s.UID))

	return hex.EncodeToString(sum[:])[:8]
}

// rotationDue reports whether less than 20% of s's token lifetime is left, and
// when that point is reached. An unreadable expiry counts as due.
func rotationDue(s *corev1.Secret, now time.Time) (bool, time.Time) {
	exp, err := time.Parse(time.RFC3339, s.Annotations[AnnotationExpiresAt])
	if err != nil {
		return true, now
	}
	created := s.CreationTimestamp.Time
	at := created.Add(exp.Sub(created) * 8 / 10)

	return !now.Before(at), at
}

// rotationRequested reports whether t asks for a rotation s has not answered.
func rotationRequested(t *solarv1alpha1.Target, s *corev1.Secret) bool {
	want := t.Annotations[AnnotationRotateAgentCredential]

	return want != "" && want != s.Annotations[AnnotationRotationRequest]
}

// reportShowsCredential reports whether the agent runs a chart rendered with
// s: the agent release's live chart version carries credentialTag(s).
func reportShowsCredential(report *solarv1alpha1.TargetReport, s *corev1.Secret) bool {
	tag := "-" + credentialTag(s)
	for _, rel := range report.Spec.Releases {
		if strings.HasSuffix(rel.Revision, tag) {
			return true
		}
	}

	return false
}

// mapAgentCredentialSecretToTarget enqueues the Target a credential Secret belongs to.
func mapAgentCredentialSecretToTarget(_ context.Context, obj client.Object) []reconcile.Request {
	target, ok := obj.GetLabels()[LabelAgentCredential]
	if !ok {
		return nil
	}

	return []reconcile.Request{{Namespace: obj.GetNamespace(), Name: target}}
}
