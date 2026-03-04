// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package zot

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	containerTimeout = time.Second * 30
)

func TestContainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Zot")
}

var _ = Describe("Zot", func() {
	container, err := New(containerTimeout)
	Expect(err).NotTo(HaveOccurred())
	Expect(container).NotTo(BeNil())

	It("should successfully run the container", func() {
		err = container.WaitFor(containerTimeout)
		Expect(err).NotTo(HaveOccurred())

		fmt.Fprintf(GinkgoWriter, "container is running. Port %d", container.GetPort())
	})
})
