// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package ocmv2

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOCMv2(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OCM v2 Suite")
}
