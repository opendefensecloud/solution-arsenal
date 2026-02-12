// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

// SplitRepository splits the repository into its base and component descriptor part.
func SplitRepository(repo string) (string, string, error) {
	const separator = "/component-descriptors/"

	parts := strings.Split(repo, separator)

	if len(parts) != 2 {
		return "", "", fmt.Errorf(
			"repository is not a component descriptor: splitting '%s' at '%s'"+
				"returns %d parts, expected exactly 2", repo, separator, len(parts),
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

	reg := regexp.MustCompile("[^a-z0-9]+")
	name = reg.ReplaceAllString(name, "-")

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
