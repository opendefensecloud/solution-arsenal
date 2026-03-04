// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package docker

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	repo = "nginx"
	tag  = "mainline-alpine"
)

var (
	containerTimeout = time.Second * 3
	exposedPort      = "80/tcp"
)

func TestContainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "docker")
}

var _ = Describe("Pool", func() {
	const name = "nginx"

	pool, err := NewPool()
	Expect(err).NotTo(HaveOccurred())

	container, err := pool.NewContainer(name, repo, tag, containerTimeout)

	It("should successfully run the container", func() {
		err = container.WaitFor(exposedPort, containerTimeout)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should return the correct port", func() {
		port, err := container.GetPort(exposedPort)
		Expect(err).NotTo(HaveOccurred())

		Expect(port).To(Not(BeZero()))
	})

	It("should destroy the container after expiration", func() {
		Eventually(func() error {
			return container.WaitFor(exposedPort, containerTimeout)
		}, containerTimeout*2, time.Second).Should(HaveOccurred())
	})
})
