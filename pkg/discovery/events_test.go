// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"github.com/go-logr/logr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Publish", func() {
	It("delivers the event when the channel has capacity", func() {
		ch := make(chan int, 1)
		log := logr.Discard()

		Publish(&log, ch, 42)

		Expect(ch).To(Receive(Equal(42)))
	})

	It("drops the event instead of blocking when the channel is full", func() {
		ch := make(chan int, 1)
		ch <- 1
		log := logr.Discard()

		Expect(func() { Publish(&log, ch, 2) }).NotTo(Panic())
		Expect(ch).To(Receive(Equal(1)))
		Expect(ch).To(BeEmpty())
	})
})
