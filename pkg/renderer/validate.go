// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"k8s.io/apimachinery/pkg/util/validation"

	solarv1alpha1 "go.opendefense.cloud/solar/api/solar/v1alpha1"
)

// The chart templates quote every value they print, so validation is not what
// keeps input from adding manifests. It rejects values no manifest could carry
// as a render error, before a signed chart reaches a target.

func validateReleaseConfig(c solarv1alpha1.ReleaseConfig) error {
	var errs []error
	if c.TargetNamespace != "" {
		errs = append(errs, dnsLabel("targetNamespace", c.TargetNamespace))
	}
	errs = append(errs, dnsLabel("component name", c.Input.Component.Name))
	for k, r := range c.Input.Resources {
		errs = append(errs, validateResource("resource "+k, r))
		if err := validateTag(r.Tag); err != nil {
			errs = append(errs, fmt.Errorf("resource %s: tag: %w", k, err))
		}
	}

	return errors.Join(errs...)
}

func validateBootstrapConfig(c solarv1alpha1.BootstrapConfig) error {
	var errs []error
	for k, r := range c.Input.Releases {
		errs = append(errs, dnsLabel("release "+k, k), validateResource("release "+k, r))
	}

	return errors.Join(errs...)
}

func validateResource(field string, r solarv1alpha1.ResolvedResourceAccess) error {
	var errs []error
	if _, err := name.NewRepository(r.Repository); err != nil {
		errs = append(errs, fmt.Errorf("%s: repository: %w", field, err))
	}
	if r.PullSecretName != "" {
		if msgs := validation.IsDNS1123Subdomain(r.PullSecretName); len(msgs) > 0 {
			errs = append(errs, fmt.Errorf("%s: pullSecretName %q: %s", field, r.PullSecretName, strings.Join(msgs, ", ")))
		}
	}

	return errors.Join(errs...)
}

// validateTag accepts the two forms ResolvedResourceAccess.Tag takes in a
// release: an OCI tag, or "@<digest>".
func validateTag(tag string) error {
	if digest, ok := strings.CutPrefix(tag, "@"); ok {
		_, err := name.NewDigest("x@" + digest)
		return err
	}
	if tag == "" {
		return errors.New("must not be empty")
	}
	// NewTag falls back to "latest" when the tag holds a "/", so compare.
	t, err := name.NewTag("x:" + tag)
	if err == nil && t.TagStr() != tag {
		err = fmt.Errorf("invalid tag %q", tag)
	}

	return err
}

func dnsLabel(field, value string) error {
	if msgs := validation.IsDNS1123Label(value); len(msgs) > 0 {
		return fmt.Errorf("%s %q: %s", field, value, strings.Join(msgs, ", "))
	}

	return nil
}
