// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"encoding/json"
	"log"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to encode JSON response: %v", err)
	}
}

func writeK8sError(w http.ResponseWriter, err error) {
	if statusErr, ok := err.(*apierrors.StatusError); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(int(statusErr.ErrStatus.Code))
		if encErr := json.NewEncoder(w).Encode(statusErr.ErrStatus); encErr != nil {
			log.Printf("failed to encode error response: %v", encErr)
		}

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
