// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"strings"
	"testing"
)

func TestTruncateName(t *testing.T) {
	t.Parallel()

	t.Run("returns the name unchanged when it fits", func(t *testing.T) {
		t.Parallel()
		if got := truncateName("short-name", 63); got != "short-name" {
			t.Errorf("got %q, want %q", got, "short-name")
		}
	})

	t.Run("truncates and appends a hash suffix when too long", func(t *testing.T) {
		t.Parallel()
		name := strings.Repeat("a", 100)
		got := truncateName(name, 63)
		if len(got) != 63 {
			t.Errorf("got length %d, want 63", len(got))
		}
		if !strings.HasPrefix(got, strings.Repeat("a", 54)+"-") {
			t.Errorf("got %q, want prefix of 54 a's followed by '-'", got)
		}
	})

	t.Run("is deterministic for the same input", func(t *testing.T) {
		t.Parallel()
		name := strings.Repeat("b", 100)
		first := truncateName(name, 63)
		second := truncateName(name, 63)
		if first != second {
			t.Error("truncateName should be deterministic")
		}
	})

	t.Run("clamps maxLen below 10 up to 10", func(t *testing.T) {
		t.Parallel()
		name := strings.Repeat("c", 20)
		got := truncateName(name, 3)
		if len(got) != 10 {
			t.Errorf("got length %d, want 10", len(got))
		}
	})
}
