// Copyright 2026 BWI GmbH and Artefact Conduit contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
)

const (
	discoveryFinalizer    = "solar.opendefense.cloud/discovery-finalizer"
	workerClusterRoleName = "solar-controller-manager"
)

// DiscoveryReconciler reconciles a Discovery object
type DiscoveryReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      events.EventRecorder
	WorkerImage   string
	WorkerCommand string
	WorkerArgs    []string
}

//nolint:lll
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=discoveries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=discoveries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=discoveries/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile moves the current state of the cluster closer to the desired state
func (r *DiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("Discovery is being reconciled", "req", req)

	// Fetch the Order instance
	res := &solarv1alpha1.Discovery{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrlResult, nil
		}

		return ctrlResult, errLogAndWrap(log, err, "failed to get object")
	}

	// Handle deletion: cleanup artifact workflows, then remove finalizer
	if !res.DeletionTimestamp.IsZero() {
		log.V(1).Info("Discovery is being deleted")
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "Deleting", "Delete", "Discovery is being deleted, cleaning up worker")

		// Cleanup worker resources, if exists
		if err := r.deleteWorkerResources(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to clean up worker resources")
		}

		// Remove finalizer
		if slices.Contains(res.Finalizers, discoveryFinalizer) {
			log.V(1).Info("Removing finalizer from resource")
			res.Finalizers = slices.DeleteFunc(res.Finalizers, func(f string) bool {
				return f == discoveryFinalizer
			})
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to remove finalizer")
			}
		}

		return ctrlResult, nil
	}

	// Add finalizer if not present and not deleting
	if res.DeletionTimestamp.IsZero() {
		if !slices.Contains(res.Finalizers, discoveryFinalizer) {
			log.V(1).Info("Adding finalizer to resource")
			res.Finalizers = append(res.Finalizers, discoveryFinalizer)
			if err := r.Update(ctx, res); err != nil {
				return ctrlResult, errLogAndWrap(log, err, "failed to add finalizer")
			}
			// Return without requeue; the Update event will trigger reconciliation again
			return ctrlResult, nil
		}
	}

	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: discoveryPrefixed(res.Name), Namespace: res.Namespace}, pod)
	if err != nil && !apierrors.IsNotFound(err) {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "PodNotFound", "GetPod", "Failed to get pod", err)
		return ctrlResult, errLogAndWrap(log, err, "failed to get pod information")
	}

	// No pod yet, create it.
	if apierrors.IsNotFound(err) {
		if err := r.createWorkerResources(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to create pod")
		}

		return ctrlResult, nil
	}

	// Pod exists, check if it's up to date with our configuration and if it is healthy.
	if res.Status.PodGeneration != res.GetGeneration() {
		// Recreate pod, configuration mismatch
		r.Recorder.Eventf(res, nil, corev1.EventTypeNormal, "ConfigurationChanged", "CompareConfiguration", "Configuration changed. Replacing pod.")
		if err := r.deleteWorkerResources(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to clean up worker resources")
		}

		if err := r.createWorkerResources(ctx, res); err != nil {
			return ctrlResult, errLogAndWrap(log, err, "failed to create pod")
		}

		return ctrlResult, nil
	} else {
		log.V(1).Info("Configuration hasn't changed", "podGen", res.Status.PodGeneration, "gen", res.GetGeneration())
	}

	return ctrlResult, nil
}

// deleteWorkerResources deletes the resources of the worker pod
func (r *DiscoveryReconciler) deleteWorkerResources(ctx context.Context, res *solarv1alpha1.Discovery) error {
	log := ctrl.LoggerFrom(ctx)

	if err := r.Delete(ctx, &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: discoveryPrefixed(res.Name), Namespace: res.Namespace}}); err != nil && !apierrors.IsNotFound(err) {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "ServiceDeletionFailed", "DeleteService", "Failed to delete service", err)
		return errLogAndWrap(log, err, "service deletion failed")
	}

	if err := r.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: discoveryPrefixed(res.Name), Namespace: res.Namespace}}); err != nil && !apierrors.IsNotFound(err) {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "SecretDeletionFailed", "DeleteSecret", "Failed to delete secret", err)
		return errLogAndWrap(log, err, "secret deletion failed")
	}

	if err := r.Delete(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: discoveryPrefixed(res.Name), Namespace: res.Namespace}}); err != nil && !apierrors.IsNotFound(err) {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "PodDeletionFailed", "DeletePod", "Failed to delete pod", err)
		return errLogAndWrap(log, err, "pod deletion failed")
	}

	return nil
}

// createWorkerResources creates the necessary resources for the worker pod
func (r *DiscoveryReconciler) createWorkerResources(ctx context.Context, res *solarv1alpha1.Discovery) error {
	log := ctrl.LoggerFrom(ctx)

	// Create or get service account in the discovery's namespace
	workerSA := &corev1.ServiceAccount{
		ObjectMeta: objectMeta(res),
	}

	existingSA := &corev1.ServiceAccount{}
	err := r.Get(ctx, types.NamespacedName{Name: workerSA.Name, Namespace: workerSA.Namespace}, existingSA)
	if err != nil && !apierrors.IsNotFound(err) {
		return errLogAndWrap(log, err, "failed to get service account")
	}

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, workerSA); err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "ServiceAccountCreationFailed", "CreateServiceAccount", "Failed to create service account", err)
			return errLogAndWrap(log, err, "failed to create service account")
		}
		r.Recorder.Eventf(res, workerSA, corev1.EventTypeNormal, "ServiceAccountCreated", "CreateServiceAccount", "Service account created")
		if err := controllerutil.SetControllerReference(res, workerSA, r.Scheme); err != nil {
			return errLogAndWrap(log, err, "failed to set controller reference on service account")
		}
	}

	// Create ClusterRoleBinding to grant RBAC permissions to the worker service account
	clusterRoleBindingName := fmt.Sprintf("%s-worker", discoveryPrefixed(res.Name))
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "solar-discovery-controller",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     workerClusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      discoveryPrefixed(res.Name),
				Namespace: res.Namespace,
			},
		},
	}

	existingCRB := &rbacv1.ClusterRoleBinding{}
	err = r.Get(ctx, types.NamespacedName{Name: clusterRoleBindingName}, existingCRB)
	if err != nil && !apierrors.IsNotFound(err) {
		return errLogAndWrap(log, err, "failed to get clusterrolebinding")
	}

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, clusterRoleBinding); err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "ClusterRoleBindingCreationFailed", "CreateClusterRoleBinding", "Failed to create clusterrolebinding", err)
			return errLogAndWrap(log, err, "failed to create clusterrolebinding")
		}
		r.Recorder.Eventf(res, clusterRoleBinding, corev1.EventTypeNormal, "ClusterRoleBindingCreated", "CreateClusterRoleBinding", "ClusterRoleBinding created")
	} else {
		needsUpdate := false
		if existingCRB.RoleRef.Name != workerClusterRoleName {
			existingCRB.RoleRef.Name = workerClusterRoleName
			needsUpdate = true
		}
		if len(existingCRB.Subjects) != 1 ||
			existingCRB.Subjects[0].Kind != "ServiceAccount" ||
			existingCRB.Subjects[0].Name != discoveryPrefixed(res.Name) ||
			existingCRB.Subjects[0].Namespace != res.Namespace {
			existingCRB.Subjects = clusterRoleBinding.Subjects
			needsUpdate = true
		}

		if needsUpdate {
			if err := r.Update(ctx, existingCRB); err != nil {
				r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "ClusterRoleBindingUpdateFailed", "UpdateClusterRoleBinding", "Failed to update clusterrolebinding", err)
				return errLogAndWrap(log, err, "failed to update clusterrolebinding")
			}
			r.Recorder.Eventf(res, existingCRB, corev1.EventTypeNormal, "ClusterRoleBindingUpdated", "UpdateClusterRoleBinding", "ClusterRoleBinding updated")
		}
	}

	// Create secret
	secret := &corev1.Secret{
		ObjectMeta: objectMeta(res),
	}

	rp := discovery.NewRegistryProvider()
	reg := &discovery.Registry{
		Name:      res.Name,
		PlainHTTP: res.Spec.Registry.PlainHTTP,
		Hostname:  res.Spec.Registry.RegistryURL,
	}
	if res.Spec.Webhook != nil {
		reg.WebhookPath = res.Spec.Webhook.Path
		reg.Flavor = res.Spec.Webhook.Flavor
	}
	if res.Spec.DiscoveryInterval != nil {
		reg.ScanInterval = res.Spec.DiscoveryInterval.Duration
	}
	if err := rp.Register(reg); err != nil {
		return errLogAndWrap(log, err, "failed to register registry")
	}

	// Add credentials if specified
	if res.Spec.Registry.SecretRef.Name != "" {
		sec := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{Name: res.Spec.Registry.SecretRef.Name, Namespace: res.Namespace}, sec); err != nil {
			return errLogAndWrap(log, err, "failed to get registry secret")
		} else {
			username, okUser := sec.Data["username"]
			password, okPass := sec.Data["password"]
			if okUser && okPass {
				reg.Credentials = &discovery.RegistryCredentials{
					Username: string(username),
					Password: string(password),
				}
			} else {
				return fmt.Errorf("registry secret is missing username or password fields")
			}
		}
	}

	confData, err := rp.Marshal()
	if err != nil {
		return errLogAndWrap(log, err, "failed to marshal registry configuration")
	}
	secret.StringData = map[string]string{
		"config.yaml": string(confData),
	}

	existingSecret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, existingSecret)
	if err != nil && !apierrors.IsNotFound(err) {
		return errLogAndWrap(log, err, "failed to get secret")
	}

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, secret); err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "SecretCreationFailed", "CreateSecret", "Failed to create secret", err)
			return errLogAndWrap(log, err, "failed to create secret")
		}
		r.Recorder.Eventf(res, secret, corev1.EventTypeNormal, "SecretCreated", "CreateSecret", "Secret created")

		if err := controllerutil.SetControllerReference(res, secret, r.Scheme); err != nil {
			return errLogAndWrap(log, err, "failed to set controller reference")
		}
	} else {
		existingSecret.StringData = secret.StringData
		if err := r.Update(ctx, existingSecret); err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "SecretUpdateFailed", "UpdateSecret", "Failed to update secret", err)
			return errLogAndWrap(log, err, "failed to update secret")
		}
		r.Recorder.Eventf(res, existingSecret, corev1.EventTypeNormal, "SecretUpdated", "UpdateSecret", "Secret updated")

		if err := controllerutil.SetControllerReference(res, existingSecret, r.Scheme); err != nil {
			return errLogAndWrap(log, err, "failed to set controller reference")
		}
	}

	// Create pod
	var args = r.WorkerArgs
	args = append(args, "--config", "/etc/worker/config.yaml")
	pod := &corev1.Pod{
		ObjectMeta: objectMeta(res),
		Spec: corev1.PodSpec{
			ServiceAccountName: discoveryPrefixed(res.Name),
			Containers: []corev1.Container{
				{
					Name:    "worker",
					Image:   r.WorkerImage,
					Command: []string{r.WorkerCommand},
					Args:    args,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							ReadOnly:  true,
							MountPath: "/etc/worker"},
					},
					Ports: []corev1.ContainerPort{
						{
							Name:          "webhook",
							ContainerPort: 8080,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: discoveryPrefixed(res.Name),
						},
					},
				},
			},
		},
	}

	// Set owner references
	if err := controllerutil.SetControllerReference(res, pod, r.Scheme); err != nil {
		return errLogAndWrap(log, err, "failed to set controller reference")
	}

	// Create pod in cluster
	if err := r.Create(ctx, pod); err != nil {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "PodCreationFailed", "CreatePod", "Failed to create pod", err)
		return errLogAndWrap(log, err, "failed to create pod")
	}
	r.Recorder.Eventf(res, pod, corev1.EventTypeNormal, "PodCreated", "CreatePod", "Pod created")
	log.V(1).Info("Pod created", "podGen", res.GetGeneration())

	// Create or update service
	svc := &corev1.Service{
		ObjectMeta: objectMeta(res),
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Ports:    []corev1.ServicePort{{Name: "webhook", Port: 8080, TargetPort: intstr.FromString("webhook")}},
			Selector: map[string]string{"app.kubernetes.io/name": discoveryPrefixed(res.Name)},
		},
	}

	existingSvc := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, existingSvc)
	if err != nil && !apierrors.IsNotFound(err) {
		return errLogAndWrap(log, err, "failed to get service")
	}

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, svc); err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "ServiceCreationFailed", "CreateService", "Failed to create service", err)
			return errLogAndWrap(log, err, "failed to create service")
		}
		r.Recorder.Eventf(res, svc, corev1.EventTypeNormal, "ServiceCreated", "CreateService", "Service created")
	} else {
		existingSvc.Spec = svc.Spec
		if err := r.Update(ctx, existingSvc); err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "ServiceUpdateFailed", "UpdateService", "Failed to update service", err)
			return errLogAndWrap(log, err, "failed to update service")
		}
		r.Recorder.Eventf(res, existingSvc, corev1.EventTypeNormal, "ServiceUpdated", "UpdateService", "Service updated")
	}

	// Update discovery version in status
	res.Status.PodGeneration = res.GetGeneration()
	if err := r.Status().Update(ctx, res); err != nil {
		return errLogAndWrap(log, err, "failed to update status")
	}

	return nil
}

func objectMeta(res *solarv1alpha1.Discovery) metav1.ObjectMeta {
	labels := res.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["app.kubernetes.io/managed-by"] = "solar-discovery-controller"
	labels["app.kubernetes.io/component"] = "discovery-worker"
	labels["app.kubernetes.io/instance"] = res.Name
	labels["app.kubernetes.io/name"] = discoveryPrefixed(res.Name)

	return metav1.ObjectMeta{
		Name:        discoveryPrefixed(res.Name),
		Namespace:   res.Namespace,
		Labels:      labels,
		Annotations: res.Annotations,
	}
}

// discoveryPrefixed returns the name of the discovery prefixed resource
func discoveryPrefixed(discoveryName string) string {
	return fmt.Sprintf("discovery-%s", discoveryName)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&solarv1alpha1.Discovery{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
