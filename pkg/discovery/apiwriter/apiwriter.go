// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package apiwriter

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v7"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"ocm.software/ocm/api/datacontext"
	"ocm.software/ocm/api/oci"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociartifact"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/relativeociref"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/naming"
)

const (
	componentLabel = "solar.opendefense.cloud/component"
)

var _ discovery.Processor[discovery.WriteAPIResourceEvent, any] = &APIWriter{}

type APIWriter struct {
	*discovery.Runner[discovery.WriteAPIResourceEvent, any]

	client    v1alpha1.SolarV1alpha1Interface
	namespace string
	provider  *discovery.RegistryProvider
}

func NewAPIWriter(
	client v1alpha1.SolarV1alpha1Interface,
	namespace string,
	provider *discovery.RegistryProvider,
	in <-chan discovery.WriteAPIResourceEvent,
	err chan<- discovery.ErrorEvent,
	opts ...discovery.RunnerOption[discovery.WriteAPIResourceEvent, any],
) *APIWriter {

	p := &APIWriter{
		client:    client,
		namespace: namespace,
		provider:  provider,
	}
	p.Runner = discovery.NewRunner(p, in, nil, err)

	for _, opt := range opts {
		opt(p.Runner)
	}

	return p
}

func (rs *APIWriter) Process(ctx context.Context, ev discovery.WriteAPIResourceEvent) ([]any, error) {
	var op backoff.Operation[struct{}]

	switch ev.Source.Source.Type {
	case discovery.EventCreated, discovery.EventUpdated:
		ref, err := rs.getOciRef(ev)
		if err != nil {
			return nil, err
		}
		spec := ev.ComponentSpec
		op = func() (struct{}, error) { return struct{}{}, rs.ensureComponentVersion(ctx, ref, spec, ev) }
	case discovery.EventDeleted:
		op = func() (struct{}, error) { return struct{}{}, rs.requestRecheck(ctx, ev) }
	default:
		return nil, fmt.Errorf("SHOULD NOT HAPPEN: Invalid event type: %s", ev.Source.Source.Type)
	}

	// Retry selected operation if a backoff is configured
	if opts := rs.RetryOptions(); opts != nil {
		_, err := backoff.Retry(ctx, op, opts...)
		return nil, err
	}

	_, err := op()

	return nil, err
}

func (rs *APIWriter) ensureComponentVersion(ctx context.Context, ref oci.RefSpec, spec compdesc.ComponentSpec, ev discovery.WriteAPIResourceEvent) error {
	octx := ocm.New(datacontext.MODE_SHARED)
	defer func() { _ = octx.Finalize() }()

	// Get Resources
	resources := map[string]solarv1alpha1.ResourceAccess{}
	for _, res := range spec.Resources {
		acc, err := octx.AccessSpecForSpec(res.GetAccess())
		if err != nil {
			return fmt.Errorf("failed to parse access spec for resource %s: %w", res.Name, err)
		}

		var rawRef string
		switch typed := acc.(type) {
		// NOTE: Currently only ociReference and relativeOciReference are supported
		case *ociartifact.AccessSpec:
			rawRef = typed.ImageReference
		case *relativeociref.AccessSpec:
			u := url.URL{
				Scheme: ref.Scheme,
				Host:   ref.Host,
				Path:   typed.Reference,
			}
			rawRef = u.String()
		default:
			rs.Logger().Info("Unsupported access type, skipping resource (will be an error in the future)", "type", acc.GetType())
			continue
		}

		ociref, err := oci.ParseRef(rawRef)
		if err != nil {
			return fmt.Errorf("failed to parse OCI reference %q: %w", rawRef, err)
		}

		resources[res.Name] = rs.newResourceAccess(ociref)
	}

	// Attach Helm metadata to the discovered chart resource
	if ev.HelmDiscovery.ResourceName != "" {
		if ra, ok := resources[ev.HelmDiscovery.ResourceName]; ok {
			ra.Helm = &solarv1alpha1.HelmResourceMetadata{
				Name:        ev.HelmDiscovery.Name,
				Description: ev.HelmDiscovery.Description,
				Version:     ev.HelmDiscovery.Version,
				AppVersion:  ev.HelmDiscovery.AppVersion,
			}
			resources[ev.HelmDiscovery.ResourceName] = ra
		}
	}

	// Get Entrypoint
	entrypoint := solarv1alpha1.Entrypoint{}
	if ev.HelmDiscovery.ResourceName != "" {
		entrypoint.ResourceName = ev.HelmDiscovery.ResourceName
		entrypoint.Type = solarv1alpha1.EntrypointTypeHelm
	}
	// NOTE: Currently only helm is supported as Entrypoint

	// Validate Entrypoint
	if _, ok := resources[entrypoint.ResourceName]; entrypoint.ResourceName != "" && !ok {
		return fmt.Errorf("entrypoint `%s` was not provided in resource map", entrypoint.ResourceName)
	}

	comp := naming.SanitizeWithHash(spec.Name)

	cv := &solarv1alpha1.ComponentVersion{
		Name: naming.ComponentVersionName(spec.Name, ref.Version()),
		Labels: map[string]string{
			componentLabel: comp,
		},
		Spec: solarv1alpha1.ComponentVersionSpec{
			ComponentName: spec.Name,
			Scheme:        ref.Scheme,
			Registry:      ref.Host,
			Repository:    ref.Repository,
			Tag:           ref.Version(),
			Digest:        ev.Source.Source.Digest,
			Resources:     resources,
			Entrypoint:    entrypoint,
		},
		// Only honoured on Create: discovery just read the descriptor, so the artifact
		// exists. On Update the apiserver keeps the controller-owned status.
		Status: solarv1alpha1.ComponentVersionStatus{
			Conditions: []metav1.Condition{{
				Type:               solarv1alpha1.ConditionTypeAvailable,
				Status:             metav1.ConditionTrue,
				Reason:             solarv1alpha1.ReasonDescriptorAvailable,
				Message:            "discovered in registry",
				LastTransitionTime: metav1.Now(),
			}},
		},
	}

	_, err := rs.client.ComponentVersions(rs.namespace).Create(ctx, cv, metav1.CreateOptions{})
	if err != nil && errors.IsAlreadyExists(err) {
		existing, getErr := rs.client.ComponentVersions(rs.namespace).Get(ctx, cv.Name, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("failed to get existing component version for update: %w", getErr)
		}

		// An OCM component version is immutable, so a different descriptor digest
		// under the same name means the tag was re-pushed with other content. Refuse
		// to re-point the ComponentVersion at it: the controller then reports the
		// recorded descriptor as missing, which is the signal an operator needs.
		// The scan path reaches the same outcome by never updating existing CVs.
		if existing.Spec.Digest != "" && existing.Spec.Digest != cv.Spec.Digest {
			rs.Logger().Info("refusing to update component version with a different descriptor digest; delete it to re-discover",
				"name", cv.Name, "recorded", existing.Spec.Digest, "found", cv.Spec.Digest)

			return nil
		}

		cv.ResourceVersion = existing.ResourceVersion
		_, err = rs.client.ComponentVersions(rs.namespace).Update(ctx, cv, metav1.UpdateOptions{})
	}

	return err
}

// requestRecheck handles a registry delete event.
// The matching ComponentVersions are only annotated; the annotation change
// makes the controller re-check availability immediately instead of waiting
// for the next interval.
func (rs *APIWriter) requestRecheck(ctx context.Context, ev discovery.WriteAPIResourceEvent) error {
	digest := ev.Source.Source.Digest
	if digest == "" {
		rs.Logger().V(1).Info("delete event without digest, leaving it to the periodic availability check",
			"repository", ev.Source.Source.Repository)

		return nil
	}

	cvList, err := rs.client.ComponentVersions(rs.namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.digest", digest).String(),
	})
	if err != nil {
		return fmt.Errorf("failed to list component versions by digest: %w", err)
	}

	patch := fmt.Appendf(nil, `{"metadata":{"annotations":{%q:%q}}}`,
		solarv1alpha1.AvailabilityRecheckAnnotation, time.Now().UTC().Format(time.RFC3339Nano))
	for _, cv := range cvList.Items {
		if cv.Spec.Digest != digest {
			continue
		}
		_, err := rs.client.ComponentVersions(rs.namespace).Patch(ctx, cv.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to request availability re-check for component version %s: %w", cv.Name, err)
		}
		rs.Logger().Info("requested availability re-check", "name", cv.Name, "digest", digest)
	}

	return nil
}

func (rs *APIWriter) newResourceAccess(ociref oci.RefSpec) solarv1alpha1.ResourceAccess {
	u := url.URL{
		Host: ociref.Host,
		Path: ociref.Repository,
	}
	repo := u.String()
	// we just stripped the scheme, now we trim the leading
	// slashes from URL.String()
	repo = strings.TrimPrefix(repo, "//")

	var insecure bool
	if ociref.Scheme == "http" {
		insecure = true
	}

	return solarv1alpha1.ResourceAccess{
		Repository: repo,
		Insecure:   insecure,
		Tag:        ociref.Version(),
	}
}

func (rs *APIWriter) getOciRef(ev discovery.WriteAPIResourceEvent) (oci.RefSpec, error) {
	registry := rs.provider.Get(ev.Source.Source.Registry)
	if registry == nil {
		rs.Logger().V(2).Info("invalid registry", "registry", ev.Source.Source.Registry)
		return oci.RefSpec{}, fmt.Errorf("invalid registry: %s", ev.Source.Source.Registry)
	}
	cvURL := fmt.Sprintf("%s/%s/%s:%s", registry.GetURL(), ev.Source.Namespace, ev.Source.Component, ev.Source.Source.Version)

	return oci.ParseRef(cvURL)
}
