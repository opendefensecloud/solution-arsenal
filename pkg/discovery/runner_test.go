// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v5"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runner.RetryOptions with backoff.Retry", func() {
	type ev struct{}
	var r *Runner[ev, any]

	BeforeEach(func() {
		r = NewRunner[ev, any](nil, nil, nil, nil)
		WithBackoff[ev, any](100*time.Microsecond, time.Millisecond, time.Second)(r)
	})

	It("retries transient failures until success", func() {
		attempts := 0
		op := func() (struct{}, error) {
			attempts++
			if attempts < 3 {
				return struct{}{}, errors.New("flaky")
			}

			return struct{}{}, nil
		}
		_, err := backoff.Retry(context.Background(), op, r.RetryOptions()...)
		Expect(err).NotTo(HaveOccurred())
		Expect(attempts).To(Equal(3))
	})

	It("stops on backoff.Permanent", func() {
		fatal := errors.New("fatal")
		attempts := 0
		op := func() (struct{}, error) {
			attempts++

			return struct{}{}, backoff.Permanent(fatal)
		}
		_, err := backoff.Retry(context.Background(), op, r.RetryOptions()...)
		Expect(attempts).To(Equal(1))
		Expect(errors.Is(err, fatal)).To(BeTrue())
	})
})
