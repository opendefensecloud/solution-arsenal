// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func generationName(res metav1.Object) string {
	return fmt.Sprintf("%s-%d", res.GetName(), res.GetGeneration())
}
