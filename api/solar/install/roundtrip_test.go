// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package install

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/apitesting/roundtrip"
	"k8s.io/apimachinery/pkg/runtime"

	"go.opendefense.cloud/solar/api/solar/fuzzer"
)

func TestRoundTripTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	Install(scheme)
	roundtrip.RoundTripTestForAPIGroup(t, Install, fuzzer.Funcs)
	// TODO: enable protobuf generation for the sample-apiserver
	// roundtrip.RoundTripProtobufTestForAPIGroup(t, Install, orderfuzzer.Funcs)
}
