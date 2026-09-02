// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v7"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fastRetryOptions keeps the intervals short so exhaustion cases finish quickly.
func fastRetryOptions(maxElapsedTime time.Duration) []backoff.RetryOption {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = time.Millisecond
	b.MaxInterval = 2 * time.Millisecond

	return []backoff.RetryOption{
		backoff.WithBackOff(b),
		backoff.WithMaxElapsedTime(maxElapsedTime),
	}
}

var _ = Describe("RetryFailure", func() {
	It("passes through an error that did not come from backoff.Retry", func() {
		plain := errors.New("plain")

		fields, logErr := RetryFailure(plain)
		Expect(logErr).To(MatchError(plain))
		Expect(fields).To(BeNil())
	})

	It("reports a permanent failure and surfaces the operation error", func() {
		fatal := errors.New("fatal")
		_, err := backoff.Retry(context.Background(), func() (struct{}, error) {
			return struct{}{}, backoff.Permanent(fatal)
		}, fastRetryOptions(time.Minute)...)

		fields, logErr := RetryFailure(err)
		Expect(logErr).To(MatchError(fatal))
		Expect(fields).To(Equal([]any{"retryCause", RetryCausePermanent}))
	})

	It("reports an exhausted max elapsed time", func() {
		transient := errors.New("transient")
		_, err := backoff.Retry(context.Background(), func() (struct{}, error) {
			return struct{}{}, transient
		}, fastRetryOptions(10*time.Millisecond)...)

		fields, logErr := RetryFailure(err)
		Expect(logErr).To(MatchError(transient))
		Expect(fields).To(Equal([]any{"retryCause", RetryCauseMaxElapsedTime}))
	})

	It("reports exhausted tries", func() {
		transient := errors.New("transient")
		opts := append(fastRetryOptions(time.Minute), backoff.WithMaxTries(2))
		_, err := backoff.Retry(context.Background(), func() (struct{}, error) {
			return struct{}{}, transient
		}, opts...)

		fields, logErr := RetryFailure(err)
		Expect(logErr).To(MatchError(transient))
		Expect(fields).To(Equal([]any{"retryCause", RetryCauseExhausted}))
	})

	It("reports a cancelled context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		transient := errors.New("transient")
		_, err := backoff.Retry(ctx, func() (struct{}, error) {
			cancel()

			return struct{}{}, transient
		}, fastRetryOptions(time.Minute)...)

		fields, logErr := RetryFailure(err)
		Expect(logErr).To(MatchError(transient))
		Expect(fields).To(Equal([]any{"retryCause", RetryCauseCanceled}))
	})
})
