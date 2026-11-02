// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

// Package admission holds solar-apiserver admission plugins.
package admission

import (
	"context"
	"errors"
	"fmt"
	"io"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/initializer"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"go.opendefense.cloud/solar/api/solar"
)

const (
	// PluginName is the name the plugin is registered and enabled under.
	PluginName = "SolarTargetCredential"
	// AnnotationTargetCredential on a Release asks SolAr to render the Target's agent
	// credential into that Release's chart. Only "true" is valid.
	AnnotationTargetCredential = "solar.opendefense.cloud/target-credential"
	// VerbInjectTargetCredential is the RBAC verb on releases needed to set the
	// annotation, and on rendertasks to reference a credential.
	VerbInjectTargetCredential = "inject-target-credential"
)

// targetCredential lets only users holding VerbInjectTargetCredential opt a
// Release into receiving a Target credential, or create a RenderTask that
// carries one. Without it anyone who may write Releases or RenderTasks could get
// a Target's token rendered into a chart they control.
type targetCredential struct {
	*admission.Handler
	authz authorizer.Authorizer
}

var (
	_ admission.ValidationInterface = &targetCredential{}
	_ initializer.WantsAuthorizer   = &targetCredential{}
)

func newPlugin() *targetCredential {
	return &targetCredential{Handler: admission.NewHandler(admission.Create, admission.Update)}
}

// Register registers the plugin. Pass it to kit's WithAdmissionPlugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(io.Reader) (admission.Interface, error) { return newPlugin(), nil })
}

func (p *targetCredential) SetAuthorizer(a authorizer.Authorizer) { p.authz = a }

func (p *targetCredential) ValidateInitialization() error {
	if p.authz == nil {
		return errors.New(PluginName + ": missing authorizer")
	}

	return nil
}

func (p *targetCredential) Validate(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	if a.GetSubresource() != "" {
		return nil
	}
	switch a.GetResource().GroupResource() {
	case solar.SchemeGroupVersion.WithResource("releases").GroupResource():
		return p.validateRelease(ctx, a)
	case solar.SchemeGroupVersion.WithResource("rendertasks").GroupResource():
		return p.validateRenderTask(ctx, a)
	default:
		return nil
	}
}

// validateRenderTask gates the other path to a token: a RenderTask naming a
// credential Secret. Its push target is in the same spec, so any spec change
// of such a task needs the verb as well.
func (p *targetCredential) validateRenderTask(ctx context.Context, a admission.Attributes) error {
	rt, ok := a.GetObject().(*solar.RenderTask)
	if !ok {
		return apierrors.NewInternalError(fmt.Errorf("%s: expected *solar.RenderTask, got %T", PluginName, a.GetObject()))
	}
	if rt.Spec.TargetCredentialSecretRef == nil && rt.Spec.ReleaseConfig.AgentCredential == nil {
		return nil
	}
	if old, ok := a.GetOldObject().(*solar.RenderTask); ok && apiequality.Semantic.DeepEqual(old.Spec, rt.Spec) {
		return nil
	}

	return p.authorize(ctx, a, "rendertasks", "setting targetCredentialSecretRef or releaseConfig.agentCredential")
}

func (p *targetCredential) validateRelease(ctx context.Context, a admission.Attributes) error {
	rel, ok := a.GetObject().(*solar.Release)
	if !ok {
		// Fail closed: a releases object of an unexpected type must not skip the gate.
		return apierrors.NewInternalError(fmt.Errorf("%s: expected *solar.Release, got %T", PluginName, a.GetObject()))
	}

	val, set := rel.Annotations[AnnotationTargetCredential]
	if !set {
		return nil
	}
	if val != "true" {
		return apierrors.NewBadRequest(fmt.Sprintf("annotation %s must be \"true\" when set, got %q",
			AnnotationTargetCredential, val))
	}

	// Keeping an annotation someone was already allowed to set needs nothing extra,
	// unless the spec changes: the Release's values override the agent connection
	// values, so changing them can redirect where the token is sent.
	if old, ok := a.GetOldObject().(*solar.Release); ok && old.Annotations[AnnotationTargetCredential] == "true" &&
		apiequality.Semantic.DeepEqual(old.Spec, rel.Spec) {
		return nil
	}

	return p.authorize(ctx, a, "releases", "setting "+AnnotationTargetCredential)
}

// authorize requires VerbInjectTargetCredential on resource for the request's object.
func (p *targetCredential) authorize(ctx context.Context, a admission.Attributes, resource, what string) error {
	decision, reason, err := p.authz.Authorize(ctx, authorizer.AttributesRecord{
		User:            a.GetUserInfo(),
		Verb:            VerbInjectTargetCredential,
		Namespace:       a.GetNamespace(),
		APIGroup:        solar.GroupName,
		Resource:        resource,
		Name:            a.GetName(),
		ResourceRequest: true,
	})
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("authorizing %s: %w", VerbInjectTargetCredential, err))
	}
	if decision != authorizer.DecisionAllow {
		return admission.NewForbidden(a, fmt.Errorf("%s requires verb %q on %s: %s",
			what, VerbInjectTargetCredential, resource, reason))
	}

	return nil
}
