// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

// Kubernetes only requires most metadata.name values to be RFC 1123
// subdomains (dots allowed, up to 253 chars), but these names are embedded in
// generated OCI artifact URLs and rendered object names (e.g. the "release-<name>"
// and "bootstrap-<target>" repository paths built in the controllers), so we
// intentionally enforce the stricter RFC 1123 label rule: no dots, up to 63 chars.
export const K8S_NAME_MAX_LENGTH = 63
export const K8S_NAME_REGEX = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/

export type K8sNameResult =
  | { ok: true; value: string }
  | { ok: false; error: string }

export function parseK8sName(raw: string, requiredError = 'Name is required.'): K8sNameResult {
  const value = raw.trim()
  if (value === '') {
    return { ok: false, error: requiredError }
  }
  if (!K8S_NAME_REGEX.test(value) || value.length > K8S_NAME_MAX_LENGTH) {
    return {
      ok: false,
      error:
        'Label must be ≤63 and contain only lowercase alphanumeric characters and "-".',
    }
  }
  return { ok: true, value }
}
