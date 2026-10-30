// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"

	"go.opendefense.cloud/solar/api/solar"
)

var releasesGVR = solar.SchemeGroupVersion.WithResource("releases")

func release(annotations map[string]string) *solar.Release {
	return &solar.Release{Name: "agent", Namespace: "ns", Annotations: annotations}
}

// allowIf answers Allow only for the inject verb, and only when allowed is true.
func allowIf(allowed bool, seen *authorizer.Attributes) authorizer.Authorizer {
	return authorizer.AuthorizerFunc(func(_ context.Context, a authorizer.Attributes) (authorizer.Decision, string, error) {
		*seen = a
		if allowed && a.GetVerb() == VerbInjectTargetCredential {
			return authorizer.DecisionAllow, "", nil
		}

		return authorizer.DecisionNoOpinion, "no rule", nil
	})
}

func validate(t *testing.T, authz authorizer.Authorizer, op admission.Operation, obj, old runtime.Object) error {
	t.Helper()
	p := newPlugin()
	p.SetAuthorizer(authz)
	if err := p.ValidateInitialization(); err != nil {
		t.Fatal(err)
	}
	attrs := admission.NewAttributesRecord(obj, old, solar.SchemeGroupVersion.WithKind("Release"),
		"ns", "agent", releasesGVR, "", op, nil, false, &user.DefaultInfo{Name: "alice"})

	return p.Validate(context.Background(), attrs, nil)
}

var on = map[string]string{AnnotationTargetCredential: "true"}

func TestCreateWithAnnotationNeedsVerb(t *testing.T) {
	var seen authorizer.Attributes
	err := validate(t, allowIf(false, &seen), admission.Create, release(on), nil)
	if !apierrors.IsForbidden(err) {
		t.Fatalf("want Forbidden, got %v", err)
	}
	if seen.GetVerb() != VerbInjectTargetCredential || seen.GetResource() != "releases" ||
		seen.GetAPIGroup() != solar.GroupName || seen.GetNamespace() != "ns" || seen.GetName() != "agent" ||
		seen.GetUser().GetName() != "alice" {
		t.Fatalf("unexpected authorizer attributes: %+v", seen)
	}
}

func TestCreateWithAnnotationAllowed(t *testing.T) {
	var seen authorizer.Attributes
	if err := validate(t, allowIf(true, &seen), admission.Create, release(on), nil); err != nil {
		t.Fatal(err)
	}
}

func TestCreateWithoutAnnotationSkipsAuthorizer(t *testing.T) {
	var seen authorizer.Attributes
	if err := validate(t, allowIf(false, &seen), admission.Create, release(nil), nil); err != nil {
		t.Fatal(err)
	}
	if seen != nil {
		t.Fatal("authorizer must not be asked")
	}
}

func TestUpdateAddingAnnotationNeedsVerb(t *testing.T) {
	var seen authorizer.Attributes
	err := validate(t, allowIf(false, &seen), admission.Update, release(on), release(nil))
	if !apierrors.IsForbidden(err) {
		t.Fatalf("want Forbidden, got %v", err)
	}
}

func TestUpdateKeepingAnnotationIsFree(t *testing.T) {
	var seen authorizer.Attributes
	if err := validate(t, allowIf(false, &seen), admission.Update, release(on), release(on)); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRemovingAnnotationIsFree(t *testing.T) {
	var seen authorizer.Attributes
	if err := validate(t, allowIf(false, &seen), admission.Update, release(nil), release(on)); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidValueRejected(t *testing.T) {
	var seen authorizer.Attributes
	err := validate(t, allowIf(true, &seen), admission.Create,
		release(map[string]string{AnnotationTargetCredential: "yes"}), nil)
	if !apierrors.IsBadRequest(err) {
		t.Fatalf("want BadRequest, got %v", err)
	}
}

func TestOtherResourcesIgnored(t *testing.T) {
	var seen authorizer.Attributes
	p := newPlugin()
	p.SetAuthorizer(allowIf(false, &seen))
	attrs := admission.NewAttributesRecord(release(on), nil, schema.GroupVersionKind{},
		"ns", "agent", solar.SchemeGroupVersion.WithResource("targets"), "", admission.Create, nil, false,
		&user.DefaultInfo{Name: "alice"})
	if err := p.Validate(context.Background(), attrs, nil); err != nil {
		t.Fatal(err)
	}
}

func TestWrongObjectTypeFailsClosed(t *testing.T) {
	var seen authorizer.Attributes
	err := validate(t, allowIf(true, &seen), admission.Create, &solar.Target{Name: "agent", Namespace: "ns"}, nil)
	if !apierrors.IsInternalError(err) {
		t.Fatalf("want InternalError, got %v", err)
	}
}

func withUniqueName(r *solar.Release, name string) *solar.Release {
	r.Spec.UniqueName = name

	return r
}

func TestUpdateAnnotatedSpecChangeNeedsVerb(t *testing.T) {
	var seen authorizer.Attributes
	err := validate(t, allowIf(false, &seen), admission.Update,
		withUniqueName(release(on), "new"), withUniqueName(release(on), "old"))
	if !apierrors.IsForbidden(err) {
		t.Fatalf("want Forbidden, got %v", err)
	}
}

func TestUpdateAnnotatedSpecChangeWithVerbAllowed(t *testing.T) {
	var seen authorizer.Attributes
	if err := validate(t, allowIf(true, &seen), admission.Update,
		withUniqueName(release(on), "new"), withUniqueName(release(on), "old")); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateAnnotatedMetadataOnlyIsFree(t *testing.T) {
	var seen authorizer.Attributes
	newRel := release(on)
	newRel.Labels = map[string]string{"a": "b"}
	if err := validate(t, allowIf(false, &seen), admission.Update, newRel, release(on)); err != nil {
		t.Fatal(err)
	}
}
