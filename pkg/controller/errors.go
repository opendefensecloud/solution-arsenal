// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
)

const (
	testModeEnvVar = "CONTROLLER_TEST_MODE"
)

var testMode = os.Getenv(testModeEnvVar) == "true"

// isNamespaceTerminatingError returns true if the error indicates that the namespace
// is being terminated. This is used to ignore errors that occur when trying to create
// resources in a namespace that is being deleted during test teardown.
// See: https://book.kubebuilder.io/reference/envtest#testing-considerations
func isNamespaceTerminatingError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(err.Error(), "because it is being terminated")
}

// errLogAndWrap is a small utility function to reduce code and be able to directly
// return errors, but wrap them and log them at the same time. It also capitalizes the
// first letter as well. Short texts will be handled.
// When CONTROLLER_TEST_MODE=true, namespace terminating errors are silently ignored
// to suppress errors that occur when a namespace is being deleted during test cleanup.
func errLogAndWrap(log logr.Logger, err error, text string) error {
	if err == nil {
		return nil
	}
	if testMode && isNamespaceTerminatingError(err) {
		return nil
	}
	textLen := len(text)
	switch textLen {
	case 0:
		return err
	case 1:
		logText := strings.ToUpper(text)
		log.Error(err, logText)

		return fmt.Errorf(text+": %w", err)
	default:
		logText := strings.ToUpper(text[:1]) + text[1:]
		log.Error(err, logText)

		return fmt.Errorf(text+": %w", err)
	}
}
