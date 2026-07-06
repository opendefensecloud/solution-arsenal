// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-logr/logr"
)

func TestIsNamespaceTerminatingError(t *testing.T) {
	t.Parallel()

	if isNamespaceTerminatingError(nil) {
		t.Error("nil error should not be a namespace terminating error")
	}

	if isNamespaceTerminatingError(errors.New("some other failure")) {
		t.Error("unrelated error should not be a namespace terminating error")
	}

	if !isNamespaceTerminatingError(errors.New("admission webhook denied: because it is being terminated")) {
		t.Error("expected the namespace-terminating message to be detected")
	}
}

func TestErrLogAndWrap(t *testing.T) {
	t.Run("returns nil for a nil error", func(t *testing.T) {
		t.Parallel()
		if err := errLogAndWrap(logr.Discard(), nil, "doing thing"); err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("returns the raw error when text is empty", func(t *testing.T) {
		t.Parallel()
		orig := errors.New("boom")
		if err := errLogAndWrap(logr.Discard(), orig, ""); !errors.Is(err, orig) {
			t.Errorf("expected wrapped error to match original, got %v", err)
		}
	})

	t.Run("capitalizes a single-character prefix", func(t *testing.T) {
		t.Parallel()
		orig := errors.New("boom")
		err := errLogAndWrap(logr.Discard(), orig, "x")
		if err == nil || !strings.HasPrefix(err.Error(), "x: ") {
			t.Errorf("got %v, want prefix 'x: '", err)
		}
	})

	t.Run("capitalizes the first letter of a longer prefix and wraps the error", func(t *testing.T) {
		t.Parallel()
		orig := errors.New("boom")
		err := errLogAndWrap(logr.Discard(), orig, "failed to do thing")
		if err == nil || !strings.HasPrefix(err.Error(), "failed to do thing: ") {
			t.Errorf("got %v, want prefix 'failed to do thing: '", err)
		}
		if !errors.Is(err, orig) {
			t.Error("expected wrapped error to preserve the original via errors.Is")
		}
	})

	t.Run("suppresses namespace terminating errors in test mode", func(t *testing.T) {
		t.Setenv(testModeEnvVar, "true")
		orig := errors.New("admission webhook denied: because it is being terminated")
		if err := errLogAndWrap(logr.Discard(), orig, "cleanup"); err != nil {
			t.Errorf("expected nil in test mode for a namespace-terminating error, got %v", err)
		}
	})

	t.Run("does not suppress namespace terminating errors outside test mode", func(t *testing.T) {
		_ = os.Unsetenv(testModeEnvVar)
		orig := errors.New("admission webhook denied: because it is being terminated")
		if err := errLogAndWrap(logr.Discard(), orig, "cleanup"); err == nil {
			t.Error("expected a wrapped error outside test mode")
		}
	})
}
