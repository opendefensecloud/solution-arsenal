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
	// VerbInjectTargetCredential is the RBAC verb on releases needed to set the annotation.
	VerbInjectTargetCredential = "inject-target-credential"
)

// targetCredential lets only users holding VerbInjectTargetCredential on a Release
// opt it into receiving a Target credential. Without it anyone who may write
// Releases could get a Target's token rendered into their own chart.
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
	if a.GetResource().GroupResource() != solar.SchemeGroupVersion.WithResource("releases").GroupResource() ||
		a.GetSubresource() != "" {
		return nil
	}

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

	decision, reason, err := p.authz.Authorize(ctx, authorizer.AttributesRecord{
		User:            a.GetUserInfo(),
		Verb:            VerbInjectTargetCredential,
		Namespace:       a.GetNamespace(),
		APIGroup:        solar.GroupName,
		Resource:        "releases",
		Name:            a.GetName(),
		ResourceRequest: true,
	})
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("authorizing %s: %w", VerbInjectTargetCredential, err))
	}
	if decision != authorizer.DecisionAllow {
		return admission.NewForbidden(a, fmt.Errorf("setting %s requires verb %q on releases: %s",
			AnnotationTargetCredential, VerbInjectTargetCredential, reason))
	}

	return nil
}
