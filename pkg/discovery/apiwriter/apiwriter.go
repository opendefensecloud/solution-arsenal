// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package apiwriter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/open-component-model/community/ocm-kit/helmvalues"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
	"go.opendefense.cloud/solar/client-go/clientset/versioned/typed/solar/v1alpha1"
	"go.opendefense.cloud/solar/pkg/discovery"
	"go.opendefense.cloud/solar/pkg/naming"
	"go.opendefense.cloud/solar/pkg/ocmv2"
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
		src, err := rs.componentSource(ev)
		if err != nil {
			return nil, err
		}
		comp := ev.Component
		op = func() (struct{}, error) { return struct{}{}, rs.ensureComponentVersion(ctx, src, comp, ev) }
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

func (rs *APIWriter) ensureComponentVersion(ctx context.Context, src componentSource, comp descruntime.Component, ev discovery.WriteAPIResourceEvent) error {
	// Get Resources. Reference resolution is shared with the renderer via
	// pkg/ocmv2, so the catalog and the values template can never disagree about
	// where a resource lives.
	resources := map[string]solarv1alpha1.ResourceAccess{}
	for i := range comp.Resources {
		res := comp.Resources[i]

		ref, reanchored, err := ocmv2.ResolveOCIReference(res, src.BaseURL())
		if err != nil {
			if ocmv2.IsNoOCIReference(err) {
				accessType := "<none>"
				if res.Access != nil {
					accessType = res.Access.GetType().String()
				}
				rs.Logger().V(1).Info("resource carries no resolvable OCI reference, skipping", "resource", res.Name, "type", accessType)

				continue
			}

			return fmt.Errorf("failed to resolve OCI reference for resource %s: %w", res.Name, err)
		}

		resources[res.Name] = rs.newResourceAccess(ref, reanchored && src.Insecure())
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

	compName := naming.SanitizeWithHash(comp.Name)

	cv := &solarv1alpha1.ComponentVersion{
		Name: naming.ComponentVersionName(comp.Name, src.VersionOrLatest()),
		Labels: map[string]string{
			componentLabel: compName,
		},
		Spec: solarv1alpha1.ComponentVersionSpec{
			ComponentName: comp.Name,
			Scheme:        src.Scheme,
			Registry:      src.Host,
			Repository:    src.Repository(),
			Tag:           src.Version,
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

func (rs *APIWriter) newResourceAccess(ref helmvalues.ImageReference, insecure bool) solarv1alpha1.ResourceAccess {
	repo := ref.Repository
	if ref.Host != "" {
		repo = ref.Host + "/" + strings.TrimPrefix(ref.Repository, "/")
	}

	return solarv1alpha1.ResourceAccess{
		Repository: repo,
		Insecure:   insecure,
		Tag:        versionSpec(ref),
	}
}

// versionSpec renders the version part of an image reference into the single
// string ResourceAccess.Tag carries.
//
// The encoding is the one the render templates decode (see
// pkg/renderer/template/release/templates/release.yaml): a leading "@" marks the
// value as a digest, and anything else is a tag. There is deliberately no
// combined "tag@digest" form.
func versionSpec(ref helmvalues.ImageReference) string {
	if ref.Digest != "" {
		return "@" + ref.Digest
	}

	if ref.Tag == "" {
		return "latest"
	}

	return ref.Tag
}

// componentSource locates the component version discovery read it from, in the
// pieces the catalog objects and reference resolution need.
type componentSource struct {
	// Scheme is "http" or "https".
	Scheme string
	// Host is the registry host, including a port when one is set.
	Host string
	// Namespace is the OCM namespace (the repository prefix) the component sits under.
	Namespace string
	// Component is the OCM component name.
	Component string
	// Version is the component version discovery resolved.
	Version string
}

// VersionOrLatest is the component version, defaulting to "latest" when the
// event carries none. OCM v1 applied the same default when parsing a reference
// without a version, and ComponentVersion names are derived from it.
func (s componentSource) VersionOrLatest() string {
	if s.Version == "" {
		return "latest"
	}

	return s.Version
}

// BaseURL is the "<host>/<namespace>" a repository is opened with, which is what
// repository-relative resource accesses resolve against.
func (s componentSource) BaseURL() string {
	return strings.TrimSuffix(s.Host+"/"+s.Namespace, "/")
}

// Repository is the OCI repository path of the component itself.
func (s componentSource) Repository() string {
	return strings.Trim(s.Namespace+"/"+s.Component, "/")
}

// Insecure reports whether the registry is served over plain HTTP.
func (s componentSource) Insecure() bool {
	return s.Scheme == "http"
}

func (rs *APIWriter) componentSource(ev discovery.WriteAPIResourceEvent) (componentSource, error) {
	registry := rs.provider.Get(ev.Source.Source.Registry)
	if registry == nil {
		rs.Logger().V(2).Info("invalid registry", "registry", ev.Source.Source.Registry)
		return componentSource{}, fmt.Errorf("invalid registry: %s", ev.Source.Source.Registry)
	}

	host, plainHTTP := ocmv2.SplitScheme(registry.GetURL())
	scheme := "https"
	if plainHTTP {
		scheme = "http"
	}

	return componentSource{
		Scheme:    scheme,
		Host:      host,
		Namespace: ev.Source.Namespace,
		Component: ev.Source.Component,
		Version:   ev.Source.Source.Version,
	}, nil
}
