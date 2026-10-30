// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

const agentCredentialFinalizer = "solar.opendefense.cloud/agent-credential-finalizer"

// AgentCredentialReconciler gives each Target an identity in this cluster for
// its solar-agent: a ServiceAccount whose Role reaches only that Target and its
// pre-created TargetReport, and numbered Secrets holding bound tokens for it.
type AgentCredentialReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// APIReader reads credential Secrets uncached, so a stale cache cannot
	// make the reconciler mint a second token.
	APIReader client.Reader
	// APIServerURL is this cluster's kube-apiserver as target clusters reach it.
	// Empty disables agent credentials.
	APIServerURL string
	// TokenTTL is requested for every token. TokenRequest defaults to one hour
	// without it; the cluster may still cap it lower.
	TokenTTL time.Duration
}

//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/finalizers,verbs=update
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=targetreports,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;delete;deletecollection
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;delete

func (r *AgentCredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	target := &solarv1alpha1.Target{}
	if err := r.Get(ctx, req.NamespacedName, target); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !target.DeletionTimestamp.IsZero() {
		if err := r.deleteAll(ctx, target); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, r.removeFinalizer(ctx, target)
	}

	// Revocation comes first: it must hold even when credentials are disabled.
	if target.Annotations[AnnotationAgentCredential] == AgentCredentialRevoked {
		if err := r.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(target.Namespace),
			client.MatchingLabels{LabelAgentCredential: target.Name}); err != nil {
			return ctrl.Result{}, fmt.Errorf("revoking credentials: %w", err)
		}

		return ctrl.Result{}, r.setCondition(ctx, target, metav1.ConditionFalse, ReasonAgentCredentialRevoked,
			"agent credential revoked by annotation "+AnnotationAgentCredential)
	}

	if r.APIServerURL == "" {
		return ctrl.Result{}, r.setCondition(ctx, target, metav1.ConditionFalse, ReasonAgentCredentialDisabled,
			"agent credentials are disabled: --agent-apiserver-url is not set")
	}

	if !slices.Contains(target.Finalizers, agentCredentialFinalizer) {
		// The update event re-triggers this reconcile.
		return ctrl.Result{}, r.addFinalizer(ctx, target)
	}

	if err := r.ensureIdentity(ctx, target); err != nil {
		return ctrl.Result{}, r.failed(ctx, target, err)
	}

	secrets, err := listAgentCredentials(ctx, r.APIReader, target)
	if err != nil {
		return ctrl.Result{}, err
	}

	var current *corev1.Secret
	if len(secrets) > 0 {
		current = &secrets[len(secrets)-1]
	}
	now := time.Now()
	if current == nil || len(current.Data[AgentCredentialTokenKey]) == 0 || rotationRequested(target, current) || rotationOverdue(current, now) {
		next := 1
		if current != nil {
			n, _ := credentialIndex(target.Name, current)
			next = n + 1
		}
		if current, err = r.mint(ctx, target, next); err != nil {
			return ctrl.Result{}, r.failed(ctx, target, err)
		}
	}

	// Older credentials stay valid until the agent runs a chart rendered with the
	// current one, so a target that is offline until its next update keeps reporting.
	if len(secrets) > 0 && secrets[0].Name != current.Name {
		report := &solarv1alpha1.TargetReport{}
		if err := r.Get(ctx, client.ObjectKey{Namespace: target.Namespace, Name: target.Name}, report); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, fmt.Errorf("reading TargetReport: %w", err)
		}
		if reportShowsCredential(report, current) {
			for i := range secrets {
				if secrets[i].Name == current.Name {
					continue
				}
				if err := r.Delete(ctx, &secrets[i]); client.IgnoreNotFound(err) != nil {
					return ctrl.Result{}, fmt.Errorf("deleting superseded credential %s: %w", secrets[i].Name, err)
				}
			}
		}
	}

	if err := r.setCondition(ctx, target, metav1.ConditionTrue, ReasonAgentCredentialIssued,
		fmt.Sprintf("credential %s expires at %s", current.Name, current.Annotations[AnnotationExpiresAt])); err != nil {
		return ctrl.Result{}, err
	}

	_, at := rotationDue(current, now)
	wait := time.Until(at)
	if wait <= 0 {
		wait = time.Second
	}

	return ctrl.Result{RequeueAfter: wait}, nil
}

func rotationOverdue(s *corev1.Secret, now time.Time) bool {
	due, _ := rotationDue(s, now)

	return due
}

// ensureIdentity creates the ServiceAccount, keeps Role and RoleBinding exact,
// and pre-creates the TargetReport the agent may only update.
func (r *AgentCredentialReconciler) ensureIdentity(ctx context.Context, t *solarv1alpha1.Target) error {
	name := agentName(t.Name)

	sa := &corev1.ServiceAccount{Name: name, Namespace: t.Namespace}
	if err := controllerutil.SetControllerReference(t, sa, r.Scheme); err != nil {
		return fmt.Errorf("owning ServiceAccount: %w", err)
	}
	if err := r.createIfMissing(ctx, sa); err != nil {
		return fmt.Errorf("creating ServiceAccount: %w", err)
	}

	role := &rbacv1.Role{Name: name, Namespace: t.Namespace}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, role, func() error {
		role.Rules = agentRules(t.Name)

		return controllerutil.SetControllerReference(t, role, r.Scheme)
	}); err != nil {
		return fmt.Errorf("ensuring Role: %w", err)
	}

	rb := &rbacv1.RoleBinding{Name: name, Namespace: t.Namespace}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, rb, func() error {
		rb.RoleRef = rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name}
		rb.Subjects = []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: name, Namespace: t.Namespace}}

		return controllerutil.SetControllerReference(t, rb, r.Scheme)
	}); err != nil {
		return fmt.Errorf("ensuring RoleBinding: %w", err)
	}

	report := &solarv1alpha1.TargetReport{Name: t.Name, Namespace: t.Namespace}
	if err := controllerutil.SetControllerReference(t, report, r.Scheme); err != nil {
		return fmt.Errorf("owning TargetReport: %w", err)
	}
	if err := r.createIfMissing(ctx, report); err != nil {
		return fmt.Errorf("pre-creating TargetReport: %w", err)
	}

	return nil
}

// createIfMissing reads from the cache first so a steady-state reconcile
// issues no write. AlreadyExists (a stale cache) is tolerated.
func (r *AgentCredentialReconciler) createIfMissing(ctx context.Context, obj client.Object) error {
	err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	if err := r.Create(ctx, obj); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

// mint creates Secret n and fills it with a token bound to that Secret, so
// deleting the Secret revokes the token. A Secret whose TokenRequest failed is
// deleted again: an empty "newest" Secret must never be rendered.
func (r *AgentCredentialReconciler) mint(ctx context.Context, t *solarv1alpha1.Target, n int) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		Name:        credentialSecretName(t.Name, n),
		Namespace:   t.Namespace,
		Labels:      map[string]string{LabelAgentCredential: t.Name},
		Annotations: map[string]string{AnnotationRotationRequest: t.Annotations[AnnotationRotateAgentCredential]},
		Type:        corev1.SecretTypeOpaque,
	}
	if err := controllerutil.SetControllerReference(t, secret, r.Scheme); err != nil {
		return nil, fmt.Errorf("owning credential Secret %s: %w", secret.Name, err)
	}
	if err := r.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("creating credential Secret %s: %w", secret.Name, err)
	}

	tr := &authenticationv1.TokenRequest{Spec: authenticationv1.TokenRequestSpec{
		ExpirationSeconds: new(int64(r.TokenTTL.Seconds())),
		BoundObjectRef: &authenticationv1.BoundObjectReference{
			Kind: "Secret", APIVersion: "v1", Name: secret.Name, UID: secret.UID,
		},
	}}
	sa := &corev1.ServiceAccount{Name: agentName(t.Name), Namespace: t.Namespace}
	if err := r.SubResource("token").Create(ctx, sa, tr); err != nil {
		return nil, errors.Join(fmt.Errorf("requesting token: %w", err), r.Delete(ctx, secret))
	}

	secret.Data = map[string][]byte{AgentCredentialTokenKey: []byte(tr.Status.Token)}
	secret.Annotations[AnnotationExpiresAt] = tr.Status.ExpirationTimestamp.UTC().Format(time.RFC3339)
	if err := r.Update(ctx, secret); err != nil {
		return nil, errors.Join(fmt.Errorf("storing token in %s: %w", secret.Name, err), r.Delete(ctx, secret))
	}

	return secret, nil
}

func (r *AgentCredentialReconciler) deleteAll(ctx context.Context, t *solarv1alpha1.Target) error {
	name := agentName(t.Name)
	if err := r.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(t.Namespace),
		client.MatchingLabels{LabelAgentCredential: t.Name}); err != nil {
		return fmt.Errorf("deleting credential Secrets: %w", err)
	}
	for _, obj := range []client.Object{
		&rbacv1.RoleBinding{Name: name, Namespace: t.Namespace},
		&rbacv1.Role{Name: name, Namespace: t.Namespace},
		&corev1.ServiceAccount{Name: name, Namespace: t.Namespace},
		&solarv1alpha1.TargetReport{Name: t.Name, Namespace: t.Namespace},
	} {
		if err := r.Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("deleting %T %s: %w", obj, obj.GetName(), err)
		}
	}

	return nil
}

func (r *AgentCredentialReconciler) addFinalizer(ctx context.Context, t *solarv1alpha1.Target) error {
	original := t.DeepCopy()
	t.Finalizers = append(t.Finalizers, agentCredentialFinalizer)
	if err := r.Patch(ctx, t, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})); err != nil {
		return fmt.Errorf("adding agent credential finalizer: %w", err)
	}

	return nil
}

func (r *AgentCredentialReconciler) removeFinalizer(ctx context.Context, t *solarv1alpha1.Target) error {
	if !slices.Contains(t.Finalizers, agentCredentialFinalizer) {
		return nil
	}
	original := t.DeepCopy()
	t.Finalizers = slices.DeleteFunc(t.Finalizers, func(s string) bool { return s == agentCredentialFinalizer })
	if err := r.Patch(ctx, t, client.MergeFromWithOptions(original, client.MergeFromWithOptimisticLock{})); err != nil {
		return fmt.Errorf("removing agent credential finalizer: %w", err)
	}

	return nil
}

func (r *AgentCredentialReconciler) failed(ctx context.Context, t *solarv1alpha1.Target, err error) error {
	// A stale view of the Secrets, not a failure: retry without MintFailed.
	if apierrors.IsAlreadyExists(err) {
		return err
	}

	return errors.Join(err, r.setCondition(ctx, t, metav1.ConditionFalse, ReasonAgentCredentialMintFailed, err.Error()))
}

func (r *AgentCredentialReconciler) setCondition(ctx context.Context, t *solarv1alpha1.Target, status metav1.ConditionStatus, reason, msg string) error {
	if !apimeta.SetStatusCondition(&t.Status.Conditions, metav1.Condition{
		Type: ConditionTypeAgentCredentialReady, Status: status, ObservedGeneration: t.Generation, Reason: reason, Message: msg,
	}) {
		return nil
	}
	if err := r.Status().Update(ctx, t); err != nil {
		return fmt.Errorf("updating %s condition: %w", ConditionTypeAgentCredentialReady, err)
	}

	return nil
}

func (r *AgentCredentialReconciler) SetupWithManager(mgr ctrl.Manager) error {
	sameNameTarget := func(_ context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{{Namespace: obj.GetNamespace(), Name: obj.GetName()}}
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("agentcredential").
		For(&solarv1alpha1.Target{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(mapAgentCredentialSecretToTarget)).
		Watches(&solarv1alpha1.TargetReport{}, handler.EnqueueRequestsFromMapFunc(sameNameTarget)).
		Complete(r)
}
