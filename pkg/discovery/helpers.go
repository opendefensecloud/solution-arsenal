// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net"
	"regexp"
	"strings"

	"ocm.software/ocm/api/credentials"
	"ocm.software/ocm/api/oci/extensions/repositories/ocireg"
	"ocm.software/ocm/api/ocm"
)

var (
	ErrNotComponentDescriptor  = errors.New("repository is not a component descriptor")
	regexNonAlphaNumericString = regexp.MustCompile("[^a-z0-9]+")
)

// SplitRepository splits the repository into its base and component descriptor part.
func SplitRepository(repo string) (string, string, error) {
	const separator = "/component-descriptors/"

	parts := strings.Split(repo, separator)

	if len(parts) != 2 {
		return "", "", fmt.Errorf(
			"%w: splitting '%s' at '%s'"+
				" returns %d parts, expected exactly 2", ErrNotComponentDescriptor, repo, separator, len(parts),
		)
	}

	return parts[0], parts[1], nil
}

// SanitizeName cleans a string to be a valid K8s resource name.
// It ensures:
// 1. Max 63 characters
// 2. Lowercase alphanumeric or '-'
// 3. Starts and ends with alphanumeric
func SanitizeName(input string) string {
	name := strings.ToLower(input)

	name = regexNonAlphaNumericString.ReplaceAllString(name, "-")

	name = strings.Trim(name, "-")

	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "-")
	}

	return name
}

// SanitizeWithHash sanitizes the input string and appends a hash if the sanitized name exceeds 63 characters.
func SanitizeWithHash(input string) string {
	clean := SanitizeName(input)

	// If the name was short enough, just return it
	if len(clean) < 57 {
		return clean
	}

	// Otherwise, use the first 57 chars + a hash of the FULL original input
	h := fnv.New32a()
	h.Write([]byte(input))
	hashParams := fmt.Sprintf("%x", h.Sum32())

	return fmt.Sprintf("%s-%s", clean[:57], hashParams)
}

// ComponentVersionName generates a name for a ComponentVersion
func ComponentVersionName(comp string, version string) string {
	return SanitizeName(fmt.Sprintf("%s-%s", comp, version))
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

// SanitizeDigestLabel converts an OCI digest (e.g. "sha256:abc123...") into a
// valid Kubernetes label value. Label values must be at most 63 characters and
// match [a-z0-9A-Z._-]. We strip the algorithm prefix and truncate the hex to
// fit, which provides sufficient uniqueness for lookup purposes.
func SanitizeDigestLabel(digest string) string {
	if digest == "" {
		return ""
	}

	// Strip the algorithm prefix (e.g. "sha256:")
	if idx := strings.Index(digest, ":"); idx >= 0 {
		digest = digest[idx+1:]
	}

	// Kubernetes label values are max 63 chars
	if len(digest) > 63 {
		digest = digest[:63]
	}

	return digest
}
