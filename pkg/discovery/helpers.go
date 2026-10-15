// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"ocm.software/ocm/api/credentials"
	"ocm.software/ocm/api/oci/extensions/repositories/ocireg"
	"ocm.software/ocm/api/ocm"
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

// FromContextWithCreds creates an OCM context with the given registry credentials
// registered for the specified hostname. The hostname must be in "host:port" format.
func FromContextWithCreds(ctx context.Context, hostname string, creds *RegistryCredentials) (ocm.Context, error) {
	octx := ocm.FromContext(ctx)
	host, port, err := net.SplitHostPort(hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to split host and port for registry %s: %s", hostname, err)
	}
	id := credentials.ConsumerIdentity{
		credentials.ATTR_TYPE: ocireg.Type,
		"hostname":            host,
		"port":                port,
	}
	ociCreds := credentials.NewCredentials(map[string]string{
		credentials.ATTR_USERNAME: creds.Username,
		credentials.ATTR_PASSWORD: creds.Password,
	})
	octx.CredentialsContext().SetCredentialsForConsumer(id, ociCreds)

	return octx, nil
}
