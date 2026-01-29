// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"fmt"
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
