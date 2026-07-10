// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/go-logr/logr"
	"golang.org/x/time/rate"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type testEvent struct{ N int }
type testOutput struct{ N int }

// testProcessor is a configurable Processor[testEvent, testOutput] used to exercise Runner
// without depending on any real discovery-pipeline stage.
type testProcessor struct {
	result []testOutput
	err    error
	calls  int
}

func (p *testProcessor) Process(_ context.Context, _ testEvent) ([]testOutput, error) {
	p.calls++

	return p.result, p.err
}

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

var _ = Describe("Runner lifecycle", func() {
	var (
		input  chan testEvent
		output chan testOutput
		errCh  chan ErrorEvent
		proc   *testProcessor
		r      *Runner[testEvent, testOutput]
	)

	BeforeEach(func() {
		input = make(chan testEvent, 1)
		output = make(chan testOutput, 1)
		errCh = make(chan ErrorEvent, 1)
		proc = &testProcessor{result: []testOutput{{N: 42}}}
		r = NewRunner[testEvent, testOutput](proc, input, output, errCh)
	})

	It("processes events from the input channel and publishes the result", func() {
		Expect(r.Start(context.Background())).To(Succeed())
		defer r.Stop()

		input <- testEvent{N: 1}

		Eventually(output).Should(Receive(Equal(testOutput{N: 42})))
	})

	It("stops cleanly and can be stopped more than once", func() {
		Expect(r.Start(context.Background())).To(Succeed())

		r.Stop()
		Expect(func() { r.Stop() }).NotTo(Panic())
	})

	It("stops the run loop when the context is cancelled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		Expect(r.Start(ctx)).To(Succeed())

		cancel()
		r.Stop()
	})
})

var _ = Describe("Runner.processEvent", func() {
	var (
		input  chan testEvent
		output chan testOutput
		errCh  chan ErrorEvent
		proc   *testProcessor
		r      *Runner[testEvent, testOutput]
	)

	BeforeEach(func() {
		input = make(chan testEvent, 1)
		output = make(chan testOutput, 1)
		errCh = make(chan ErrorEvent, 1)
		proc = &testProcessor{}
		r = NewRunner[testEvent, testOutput](proc, input, output, errCh)
	})

	It("skips publishing when the processor returns an error", func() {
		proc.err = errors.New("boom")
		r.processEvent(context.Background(), testEvent{})
		Expect(output).To(BeEmpty())
	})

	It("skips publishing when the processor returns a nil result", func() {
		proc.result = nil
		r.processEvent(context.Background(), testEvent{})
		Expect(output).To(BeEmpty())
	})

	It("does not panic when the output channel is nil", func() {
		r := NewRunner[testEvent, testOutput](proc, input, nil, errCh)
		proc.result = []testOutput{{N: 1}}
		Expect(func() { r.processEvent(context.Background(), testEvent{}) }).NotTo(Panic())
	})

	It("skips processing when the rate limiter has no capacity and the context is done", func() {
		WithRateLimiter[testEvent, testOutput](time.Hour, 0)(r)
		proc.result = []testOutput{{N: 1}}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		r.processEvent(ctx, testEvent{})
		Expect(proc.calls).To(Equal(0))
		Expect(output).To(BeEmpty())
	})

	It("processes normally once the rate limiter has capacity", func() {
		WithRateLimiter[testEvent, testOutput](time.Millisecond, 1)(r)
		proc.result = []testOutput{{N: 7}}

		r.processEvent(context.Background(), testEvent{})
		Expect(output).To(Receive(Equal(testOutput{N: 7})))
	})
})

var _ = Describe("Runner.Logger / WithLogger", func() {
	It("defaults to a discard logger", func() {
		r := NewRunner[testEvent, testOutput](&testProcessor{}, nil, nil, nil)
		Expect(r.Logger()).To(Equal(logr.Discard()))
	})

	It("applies the logger configured via WithLogger", func() {
		r := NewRunner[testEvent, testOutput](&testProcessor{}, nil, nil, nil)
		log := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))

		WithLogger[testEvent, testOutput](log)(r)

		Expect(r.Logger()).NotTo(Equal(logr.Discard()))
	})
})

var _ = Describe("WithRateLimiter", func() {
	It("configures a limiter that allows bursts up to the given size", func() {
		r := NewRunner[testEvent, testOutput](&testProcessor{}, nil, nil, nil)
		WithRateLimiter[testEvent, testOutput](time.Hour, 2)(r)

		Expect(r.rateLimiter.Burst()).To(Equal(2))
		Expect(r.rateLimiter.Limit()).To(Equal(rate.Every(time.Hour)))
	})
})
