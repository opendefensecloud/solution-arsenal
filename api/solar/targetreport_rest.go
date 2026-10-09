// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"time"

	"go.opendefense.cloud/kit/apiserver/resource"
	"go.opendefense.cloud/kit/apiserver/rest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var (
	_ resource.Object        = &TargetReport{}
	_ rest.PrepareForUpdater = &TargetReport{}
	_ rest.PrepareForCreater = &TargetReport{}
	_ rest.TableConverter    = &TargetReport{}
	_ rest.Validater         = &TargetReport{}
	_ rest.ValidateUpdater   = &TargetReport{}
)

// maxPreflightMessageLength caps a check message so one verbose check cannot
// bloat the report or the joined aggregate condition message.
const maxPreflightMessageLength = 4096

// preflightNameRE requires a PascalCase identifier for check types and
// reasons so they read as well-formed condition values.
var preflightNameRE = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)

func (o *TargetReport) GetObjectMeta() *metav1.ObjectMeta {
	return &o.ObjectMeta
}

func (o *TargetReport) NamespaceScoped() bool {
	return true
}

func (o *TargetReport) New() runtime.Object {
	return &TargetReport{}
}

func (o *TargetReport) NewList() runtime.Object {
	return &TargetReportList{}
}

func (o *TargetReport) GetGroupResource() schema.GroupResource {
	return SchemeGroupVersion.WithResource("targetreports").GroupResource()
}

func (o *TargetReport) PrepareForUpdate(_ context.Context, old runtime.Object) {
	or := old.(*TargetReport)
	incrementGenerationIfNotEqual(o, o.Spec, or.Spec)
}

func (o *TargetReport) PrepareForCreate(_ context.Context) {
	o.Generation = 1
}

func (o *TargetReport) ConvertToTable(_ context.Context, _ runtime.Object) (*metav1.Table, error) {
	passing := 0
	for _, check := range o.Spec.Preflight.Checks {
		if check.Status == metav1.ConditionTrue {
			passing++
		}
	}
	lastReport := ""
	if !o.Spec.LastReportTime.IsZero() {
		lastReport = o.Spec.LastReportTime.Format(time.RFC3339)
	}

	return newTable(o,
		[]metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Last Report", Type: "string"},
			{Name: "Checks", Type: "string"},
			{Name: "Age", Type: "string"},
		},
		[]any{
			o.Name,
			lastReport,
			fmt.Sprintf("%d/%d", passing, len(o.Spec.Preflight.Checks)),
			duration.HumanDuration(metav1.Now().Sub(o.CreationTimestamp.Time)),
		},
	), nil
}

func (o *TargetReport) Validate(_ context.Context) field.ErrorList {
	return validateTargetReport(o)
}

func (o *TargetReport) ValidateUpdate(_ context.Context, old runtime.Object) field.ErrorList {
	errs := validateTargetReport(o)

	or := old.(*TargetReport)
	if !o.Spec.LastReportTime.IsZero() && !o.Spec.LastReportTime.After(or.Spec.LastReportTime.Time) {
		errs = append(errs, field.Invalid(
			field.NewPath("spec").Child("lastReportTime"),
			o.Spec.LastReportTime,
			"must be after the previous lastReportTime",
		))
	}

	return errs
}

func validateTargetReport(o *TargetReport) field.ErrorList {
	var errs field.ErrorList
	specPath := field.NewPath("spec")

	if o.Spec.LastReportTime.IsZero() {
		errs = append(errs, field.Required(specPath.Child("lastReportTime"), "is required"))
	}

	checksPath := specPath.Child("preflight").Child("checks")
	validStatusValues := []metav1.ConditionStatus{
		metav1.ConditionTrue, metav1.ConditionFalse, metav1.ConditionUnknown,
	}
	seenChecks := map[string]struct{}{}
	for i, check := range o.Spec.Preflight.Checks {
		p := checksPath.Index(i)
		if _, ok := seenChecks[check.Type]; ok {
			errs = append(errs, field.Duplicate(p.Child("type"), check.Type))
		} else {
			seenChecks[check.Type] = struct{}{}
		}
		if !preflightNameRE.MatchString(check.Type) {
			errs = append(errs, field.Invalid(p.Child("type"), check.Type, "must be PascalCase, e.g. Crds or Namespaces"))
		}
		if !slices.Contains(validStatusValues, check.Status) {
			errs = append(errs, field.NotSupported(p.Child("status"), check.Status, validStatusValues))
		}
		if check.Reason == "" {
			errs = append(errs, field.Required(p.Child("reason"), "is required"))
		} else if !preflightNameRE.MatchString(check.Reason) {
			errs = append(errs, field.Invalid(p.Child("reason"), check.Reason, "must be PascalCase, e.g. Present or Insufficient"))
		}
		if len(check.Message) > maxPreflightMessageLength {
			errs = append(errs, field.TooLong(p.Child("message"), check.Message, maxPreflightMessageLength))
		}
	}

	releasesPath := specPath.Child("releases")
	validPhases := []string{
		string(ReleasePending), string(ReleaseProgressing),
		string(ReleaseReady), string(ReleaseDegraded), string(ReleaseFailed),
	}
	seenReleases := map[types.NamespacedName]struct{}{}
	for i, rel := range o.Spec.Releases {
		p := releasesPath.Index(i)
		key := types.NamespacedName{Name: rel.Name, Namespace: rel.Namespace}
		if rel.Name == "" {
			errs = append(errs, field.Required(p.Child("name"), "is required"))
		}
		if rel.Namespace == "" {
			errs = append(errs, field.Required(p.Child("namespace"), "is required"))
		}
		if _, ok := seenReleases[key]; ok {
			errs = append(errs, field.Duplicate(p.Child("name"), key.String()))
		} else {
			seenReleases[key] = struct{}{}
		}
		if rel.Phase == "" {
			errs = append(errs, field.Required(p.Child("phase"), "is required"))
			continue
		}
		if !slices.Contains(validPhases, string(rel.Phase)) {
			errs = append(errs, field.NotSupported(p.Child("phase"), rel.Phase, validPhases))
		}
	}

	return errs
}
