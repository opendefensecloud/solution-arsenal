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
	discoveryFinalizer = "solar.opendefense.cloud/discovery-finalizer"
	workerRoleName     = "solar-discovery-worker"
)

// DiscoveryReconciler reconciles a Discovery object
type DiscoveryReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      events.EventRecorder
	WorkerImage   string
	WorkerCommand string
	WorkerArgs    []string
	// WatchNamespace restricts reconciliation to this namespace.
	// Should be empty in production (watches all namespaces).
	// Intended for use in integration tests only.
	// See: https://book.kubebuilder.io/reference/envtest#testing-considerations
	WatchNamespace string
}

//nolint:lll
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=discoveries,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=discoveries/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=discoveries/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=components,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch;create;update;patch;delete

// needed in order to be able to grant permissions
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=components,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=solar.opendefense.cloud,resources=componentversions,verbs=get;list;watch;create;update;patch;delete

// Reconcile moves the current state of the cluster closer to the desired state
func (r *DiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	ctrlResult := ctrl.Result{}

	log.V(1).Info("Discovery is being reconciled", "req", req)

	if r.WatchNamespace != "" && req.Namespace != r.WatchNamespace {
		return ctrlResult, nil
	}

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

	if err := r.Delete(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: workerRoleName, Namespace: res.Namespace}}); err != nil && !apierrors.IsNotFound(err) {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "RoleDeletionFailed", "DeleteRole", "Failed to delete role", err)
		return errLogAndWrap(log, err, "role deletion failed")
	}

	if err := r.Delete(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: workerRoleName, Namespace: res.Namespace}}); err != nil && !apierrors.IsNotFound(err) {
		r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "RoleBindingDeletionFailed", "DeleteRoleBinding", "Failed to delete rolebinding", err)
		return errLogAndWrap(log, err, "rolebinding deletion failed")
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
		r.Recorder.Eventf(res, workerSA, corev1.EventTypeNormal, "ServiceAccountCreated", "CreateServiceAccount", "ServiceAccount created")
		if err := controllerutil.SetControllerReference(res, workerSA, r.Scheme); err != nil {
			return errLogAndWrap(log, err, "failed to set controller reference on service account")
		}
	}

	// Create Role to define RBAC permissions required for discovery worker
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workerRoleName,
			Namespace: res.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "solar-discovery-controller",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{solarv1alpha1.SchemeGroupVersion.Group},
				Resources: []string{"componentversions", "components"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}

	existingRole := &rbacv1.Role{}
	err = r.Get(ctx, types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, existingRole)
	if err != nil && !apierrors.IsNotFound(err) {
		return errLogAndWrap(log, err, "failed to get role")
	}

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, role); err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "RoleCreationFailed", "CreateRole", "Failed to create role", err)
			return errLogAndWrap(log, err, "failed to create role")
		}
		r.Recorder.Eventf(res, role, corev1.EventTypeNormal, "RoleCreated", "CreateRole", "Role created")
		if err := controllerutil.SetControllerReference(res, role, r.Scheme); err != nil {
			return errLogAndWrap(log, err, "failed to set controller reference on role")
		}
	} else {
		// check if out of sync
		needsUpdate := false
		if len(existingRole.Rules) != len(role.Rules) ||
			!slices.Equal(existingRole.Rules[0].Verbs, role.Rules[0].Verbs) ||
			!slices.Equal(existingRole.Rules[0].APIGroups, role.Rules[0].APIGroups) ||
			!slices.Equal(existingRole.Rules[0].Resources, role.Rules[0].Resources) {
			existingRole.Rules = role.Rules
			needsUpdate = true
		}
		if needsUpdate {
			if err := r.Update(ctx, existingRole); err != nil {
				r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "RoleUpdateFailed", "UpdateRole", "Failed to update role", err)
				return errLogAndWrap(log, err, "failed to update role")
			}
			r.Recorder.Eventf(res, existingRole, corev1.EventTypeNormal, "RoleUpdated", "UpdateRole", "Role updated")
		}
	}

	// Create roleBinding to grant RBAC permissions to the worker service account
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workerRoleName,
			Namespace: res.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "solar-discovery-controller",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     workerRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      workerSA.Name,
				Namespace: res.Namespace,
			},
		},
	}

	existingRB := &rbacv1.RoleBinding{}
	err = r.Get(ctx, types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, existingRB)
	if err != nil && !apierrors.IsNotFound(err) {
		return errLogAndWrap(log, err, "failed to get rolebinding")
	}

	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, roleBinding); err != nil {
			r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "RoleBindingCreationFailed", "CreateRoleBinding", "Failed to create rolebinding", err)
			return errLogAndWrap(log, err, "failed to create rolebinding")
		}
		r.Recorder.Eventf(res, roleBinding, corev1.EventTypeNormal, "RoleBindingCreated", "CreateRoleBinding", "RoleBinding created")
		if err := controllerutil.SetControllerReference(res, roleBinding, r.Scheme); err != nil {
			return errLogAndWrap(log, err, "failed to set controller reference on rolebinding")
		}
	} else {
		needsUpdate := false
		if existingRB.RoleRef.Name != workerRoleName {
			existingRB.RoleRef.Name = workerRoleName
			needsUpdate = true
		}
		if len(existingRB.Subjects) != 1 ||
			existingRB.Subjects[0].Kind != "ServiceAccount" ||
			existingRB.Subjects[0].Name != discoveryPrefixed(res.Name) ||
			existingRB.Subjects[0].Namespace != res.Namespace {
			existingRB.Subjects = roleBinding.Subjects
			needsUpdate = true
		}

		if needsUpdate {
			if err := r.Update(ctx, existingRB); err != nil {
				r.Recorder.Eventf(res, nil, corev1.EventTypeWarning, "RoleBindingUpdateFailed", "UpdateRoleBinding", "Failed to update rolebinding", err)
				return errLogAndWrap(log, err, "failed to update rolebinding")
			}
			r.Recorder.Eventf(res, existingRB, corev1.EventTypeNormal, "RoleBindingUpdated", "UpdateRoleBinding", "RoleBinding updated")
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
		Hostname:  res.Spec.Registry.Endpoint,
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
	args = append(args, "--config", "/etc/worker/config.yaml", "--namespace", res.Namespace)
	pod := &corev1.Pod{
		ObjectMeta: objectMeta(res),
		Spec: corev1.PodSpec{
			ServiceAccountName: workerSA.Name,
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

	container := corev1.Container{

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
	}

	if cmName := res.Spec.Registry.CAConfigMapRef.Name; cmName != "" {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "ca-bundle",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cmName,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "trust-bundle.pem",
							Path: "ca-bundle.pem",
						},
					},
				},
			},
		})
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "ca-bundle",
			MountPath: "/etc/ssl/certs",
			ReadOnly:  true,
		})
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "SSL_CERT_FILE",
			Value: "/etc/ssl/certs/ca-bundle.pem",
		})
	}

	pod.Spec.Containers = []corev1.Container{container}

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
