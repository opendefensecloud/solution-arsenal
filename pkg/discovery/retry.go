// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"

	"github.com/cenkalti/backoff/v7"
)

// Stable log values for the reason a backoff.Retry call stopped retrying.
//
// These are log field values, not error values. To branch on an outcome in
// code, match the sentinels directly — errors.Is(err, backoff.ErrPermanent)
// and friends — rather than comparing these strings.
const (
	RetryCausePermanent      = "permanent"
	RetryCauseMaxElapsedTime = "max-elapsed-time"
	RetryCauseExhausted      = "exhausted"
	RetryCauseCanceled       = "canceled"
	RetryCauseDeadline       = "deadline-exceeded"
	RetryCauseUnknown        = "unknown"
)

// RetryFailure splits an error returned by backoff.Retry into the parts a log
// line wants.
//
// Since backoff v6, Retry wraps every failure in a *backoff.RetryError carrying
// both the last error the operation returned and the reason retrying stopped.
// Logging that whole error produces "backoff: permanent error (last error: …)",
// which buries the actual failure and leaves the reason unqueryable. RetryFailure
// returns structured fields naming the reason, plus the operation's own error to
// log as the error:
//
//	fields, logErr := RetryFailure(err)
//	log.Error(logErr, "lookup failed", append(fields, "version", version)...)
//
// For an error that did not come from backoff.Retry it returns no fields and err
// unchanged, so callers can use it unconditionally.
func RetryFailure(err error) (fields []any, logErr error) {
	re := backoff.AsRetryError(err)
	if re == nil {
		return nil, err
	}

	logErr = re.LastErr
	if logErr == nil {
		logErr = err
	}

	return []any{"retryCause", retryCause(re)}, logErr
}

// retryCause maps a RetryError's Cause onto a stable log value. It matches
// against Cause rather than the whole error chain: LastErr is also reachable
// through Unwrap, and an operation that happened to fail with a context error
// would otherwise be misreported as a cancellation.
func retryCause(re *backoff.RetryError) string {
	switch {
	case errors.Is(re.Cause, backoff.ErrPermanent):
		return RetryCausePermanent
	case errors.Is(re.Cause, backoff.ErrMaxElapsedTime):
		return RetryCauseMaxElapsedTime
	case errors.Is(re.Cause, backoff.ErrExhausted):
		return RetryCauseExhausted
	case errors.Is(re.Cause, context.Canceled):
		return RetryCauseCanceled
	case errors.Is(re.Cause, context.DeadlineExceeded):
		return RetryCauseDeadline
	default:
		return RetryCauseUnknown
	}
}
