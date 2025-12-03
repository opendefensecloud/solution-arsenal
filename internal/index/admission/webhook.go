/*
Copyright 2024 Open Defense Cloud Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package admission

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"
)

// WebhookHandler handles admission webhook requests.
type WebhookHandler struct {
	scheme     *runtime.Scheme
	decoder    runtime.Decoder
	validators *ValidatorRegistry
	mutators   *MutatorRegistry
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(scheme *runtime.Scheme, validators *ValidatorRegistry, mutators *MutatorRegistry) *WebhookHandler {
	codecFactory := serializer.NewCodecFactory(scheme)
	return &WebhookHandler{
		scheme:     scheme,
		decoder:    codecFactory.UniversalDeserializer(),
		validators: validators,
		mutators:   mutators,
	}
}

// HandleValidate handles validating admission requests.
func (h *WebhookHandler) HandleValidate(w http.ResponseWriter, r *http.Request) {
	h.handleAdmission(w, r, false)
}

// HandleMutate handles mutating admission requests.
func (h *WebhookHandler) HandleMutate(w http.ResponseWriter, r *http.Request) {
	h.handleAdmission(w, r, true)
}

func (h *WebhookHandler) handleAdmission(w http.ResponseWriter, r *http.Request, mutating bool) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		klog.ErrorS(err, "Failed to read request body")
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Decode admission review
	review := &admissionv1.AdmissionReview{}
	if _, _, err := h.decoder.Decode(body, nil, review); err != nil {
		klog.ErrorS(err, "Failed to decode admission review")
		http.Error(w, "failed to decode admission review", http.StatusBadRequest)
		return
	}

	// Process the request
	var response *admissionv1.AdmissionResponse
	if mutating {
		response = h.mutate(r.Context(), review.Request)
	} else {
		response = h.validate(r.Context(), review.Request)
	}

	// Set the UID
	response.UID = review.Request.UID

	// Create response review
	responseReview := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: response,
	}

	// Encode and send response
	respBytes, err := json.Marshal(responseReview)
	if err != nil {
		klog.ErrorS(err, "Failed to encode admission response")
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(respBytes)
}

func (h *WebhookHandler) validate(ctx context.Context, req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	// Get the validator for this kind
	validator, ok := h.validators.Get(req.Kind.Kind)
	if !ok {
		// No validator registered, allow by default
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	// Decode the object
	obj, err := h.decodeObject(req.Object.Raw)
	if err != nil {
		return denyResponse(fmt.Sprintf("failed to decode object: %v", err))
	}

	var errs []string

	switch req.Operation {
	case admissionv1.Create:
		fieldErrs := validator.ValidateCreate(ctx, obj)
		for _, e := range fieldErrs {
			errs = append(errs, e.Error())
		}

	case admissionv1.Update:
		oldObj, err := h.decodeObject(req.OldObject.Raw)
		if err != nil {
			return denyResponse(fmt.Sprintf("failed to decode old object: %v", err))
		}
		fieldErrs := validator.ValidateUpdate(ctx, oldObj, obj)
		for _, e := range fieldErrs {
			errs = append(errs, e.Error())
		}

	case admissionv1.Delete:
		fieldErrs := validator.ValidateDelete(ctx, obj)
		for _, e := range fieldErrs {
			errs = append(errs, e.Error())
		}
	}

	if len(errs) > 0 {
		return &admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status:  "Failure",
				Message: fmt.Sprintf("validation failed: %v", errs),
				Reason:  metav1.StatusReasonInvalid,
				Code:    http.StatusUnprocessableEntity,
			},
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

func (h *WebhookHandler) mutate(ctx context.Context, req *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	// Get the mutator for this kind
	mutator, ok := h.mutators.Get(req.Kind.Kind)
	if !ok {
		// No mutator registered, allow without modification
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	// Decode the object
	obj, err := h.decodeObject(req.Object.Raw)
	if err != nil {
		return denyResponse(fmt.Sprintf("failed to decode object: %v", err))
	}

	// Apply mutations
	switch req.Operation {
	case admissionv1.Create:
		if err := mutator.MutateCreate(ctx, obj); err != nil {
			return denyResponse(fmt.Sprintf("mutation failed: %v", err))
		}

	case admissionv1.Update:
		oldObj, err := h.decodeObject(req.OldObject.Raw)
		if err != nil {
			return denyResponse(fmt.Sprintf("failed to decode old object: %v", err))
		}
		if err := mutator.MutateUpdate(ctx, oldObj, obj); err != nil {
			return denyResponse(fmt.Sprintf("mutation failed: %v", err))
		}
	}

	// Create patch
	patch, err := h.createPatch(req.Object.Raw, obj)
	if err != nil {
		return denyResponse(fmt.Sprintf("failed to create patch: %v", err))
	}

	patchType := admissionv1.PatchTypeJSONPatch
	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		Patch:     patch,
		PatchType: &patchType,
	}
}

func (h *WebhookHandler) decodeObject(raw []byte) (runtime.Object, error) {
	obj, _, err := h.decoder.Decode(raw, nil, nil)
	return obj, err
}

func (h *WebhookHandler) createPatch(original []byte, mutated runtime.Object) ([]byte, error) {
	mutatedBytes, err := json.Marshal(mutated)
	if err != nil {
		return nil, err
	}

	// Create JSON patch
	patch, err := createJSONPatch(original, mutatedBytes)
	if err != nil {
		return nil, err
	}

	return patch, nil
}

func denyResponse(message string) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Status:  "Failure",
			Message: message,
			Reason:  metav1.StatusReasonBadRequest,
			Code:    http.StatusBadRequest,
		},
	}
}

// createJSONPatch creates a JSON patch from original to modified.
func createJSONPatch(original, modified []byte) ([]byte, error) {
	// Simple implementation - in production, use a proper JSON patch library
	var origMap, modMap map[string]interface{}
	if err := json.Unmarshal(original, &origMap); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(modified, &modMap); err != nil {
		return nil, err
	}

	// For simplicity, return empty patch if no changes
	// In production, use github.com/mattbaird/jsonpatch or similar
	patch := []map[string]interface{}{}

	// Compare and create patches for changed fields
	for key, newVal := range modMap {
		if oldVal, exists := origMap[key]; !exists || !jsonEqual(oldVal, newVal) {
			patch = append(patch, map[string]interface{}{
				"op":    "replace",
				"path":  "/" + key,
				"value": newVal,
			})
		}
	}

	return json.Marshal(patch)
}

func jsonEqual(a, b interface{}) bool {
	aBytes, _ := json.Marshal(a)
	bBytes, _ := json.Marshal(b)
	return string(aBytes) == string(bBytes)
}
