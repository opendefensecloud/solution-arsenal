// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package solar

import "testing"

func TestObjectReferenceDeepCopy(t *testing.T) {
	orig := ObjectReference{Name: "foo", Namespace: "bar"}
	cp := orig.DeepCopy()
	if *cp != orig {
		t.Fatalf("DeepCopy mismatch: got %+v want %+v", *cp, orig)
	}
	cp.Namespace = "changed"
	if orig.Namespace != "bar" {
		t.Fatalf("DeepCopy did not produce an independent copy: orig mutated to %q", orig.Namespace)
	}
}
