// Copyright BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

// Package naming builds valid Kubernetes object names and label values.
package naming

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
)

var regexNonAlphaNumericString = regexp.MustCompile("[^a-z0-9]+")

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

// SanitizeWithHash sanitizes the input string and replaces the tail with a hash if the
// sanitized name is 57 characters or longer. The result is at most 63 characters.
func SanitizeWithHash(input string) string {
	clean := SanitizeName(input)

	// If the name was short enough, just return it
	if len(clean) < 57 {
		return clean
	}

	// Otherwise, use the first 54 chars + a hash of the FULL original input,
	// which keeps the result at 63 chars
	h := fnv.New32a()
	h.Write([]byte(input))
	hashParams := fmt.Sprintf("%08x", h.Sum32())

	return fmt.Sprintf("%s-%s", clean[:54], hashParams)
}

// ComponentVersionName generates a name for a ComponentVersion
func ComponentVersionName(comp string, version string) string {
	return SanitizeName(fmt.Sprintf("%s-%s", comp, version))
}
