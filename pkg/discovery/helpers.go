// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"errors"
	"fmt"
	"strings"

	"go.opendefense.cloud/solar/pkg/ocmv2"
)

var ErrNotComponentDescriptor = errors.New("repository is not a component descriptor")

// SplitRepository splits the repository into its (optional) base and component descriptor part.
func SplitRepository(repo string) (string, string, error) {
	const prefix = "component-descriptors/"
	const separator = "/component-descriptors/"

	// exit early if it's obviosly no ocm repo
	if !strings.Contains(repo, prefix) {
		return "", "", fmt.Errorf("%w: repo '%s' is not an ocm repository", ErrNotComponentDescriptor, repo)
	}

	trimmed := strings.TrimPrefix(repo, prefix)
	if trimmed != repo && len(trimmed) > 0 {
		if strings.Contains(trimmed, separator) {
			return "", "", fmt.Errorf(
				"%w: repo '%s' has multiple 'component-descriptors' separators",
				ErrNotComponentDescriptor, repo)
		}

		return "", trimmed, nil
	}

	parts := strings.Split(repo, separator)
	if len(parts) != 2 {
		return "", "", fmt.Errorf(
			"%w: repo '%s' has multiple 'component-descriptors' separators",
			ErrNotComponentDescriptor, repo)
	}

	return parts[0], parts[1], nil
}

// OCMCredentials converts discovery's registry credentials into the form
// pkg/ocmv2 opens repositories with. A nil creds yields nil, meaning anonymous.
func OCMCredentials(creds *RegistryCredentials) *ocmv2.Credentials {
	if creds == nil {
		return nil
	}

	return &ocmv2.Credentials{Username: creds.Username, Password: creds.Password}
}
