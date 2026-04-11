// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeK8sError(w http.ResponseWriter, err error) {
	if statusErr, ok := err.(*apierrors.StatusError); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(int(statusErr.ErrStatus.Code))
		_ = json.NewEncoder(w).Encode(statusErr.ErrStatus)

		return
	}

	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func listOptions() metav1.ListOptions {
	return metav1.ListOptions{}
}

func getOptions() metav1.GetOptions {
	return metav1.GetOptions{}
}

func watchOptions() metav1.ListOptions {
	return metav1.ListOptions{
		Watch: true,
	}
}
