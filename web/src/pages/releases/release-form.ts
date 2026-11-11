// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { LabelSelector, Release, ReleaseSpec } from '@/api/types'
import type { ReleasePatch } from '@/api/mutations'
import { parseK8sName } from '@/lib/k8s-name'
import { parseUserdata, stringifyUserdata } from '@/pages/profiles/profile-form-fields'

// Raw text of every Release.spec field as edited in the form.
export type ReleaseForm = {
  componentVersion: string
  componentVersionNamespace: string
  targetNamespace: string
  uniqueName: string
  antiAffinity: string
  values: string
  failedJobTTL: string
  priority: string
}

export type ReleaseFormErrors = Partial<Record<keyof ReleaseForm, string>>

export const emptyReleaseForm: ReleaseForm = {
  componentVersion: '',
  componentVersionNamespace: '',
  targetNamespace: '',
  uniqueName: '',
  antiAffinity: '',
  values: '',
  failedJobTTL: '',
  priority: '',
}

export function releaseToForm(release: Release): ReleaseForm {
  const s = release.spec
  return {
    componentVersion: s.componentVersionRef.name,
    componentVersionNamespace: s.componentVersionRef.namespace ?? '',
    targetNamespace: s.targetNamespace ?? '',
    uniqueName: s.uniqueName ?? '',
    antiAffinity: stringifyUserdata(s.antiAffinity ?? undefined),
    values: stringifyUserdata(s.values ?? undefined),
    failedJobTTL: s.failedJobTTL != null ? String(s.failedJobTTL) : '',
    priority: s.priority != null ? String(s.priority) : '',
  }
}

type Parsed<T> = { ok: true; value: T } | { ok: false; error: string }

export function parseWholeNumber(text: string): Parsed<number | undefined> {
  const t = text.trim()
  if (t === '') return { ok: true, value: undefined }
  if (!/^-?\d+$/.test(t)) return { ok: false, error: 'Must be a whole number.' }
  return { ok: true, value: Number(t) }
}

function optionalName(text: string): Parsed<string | undefined> {
  if (text.trim() === '') return { ok: true, value: undefined }
  return parseK8sName(text)
}

/** Validates the form and turns it into a Release spec; empty optional fields are left out. */
export function parseReleaseForm(
  form: ReleaseForm
): { ok: true; spec: ReleaseSpec } | { ok: false; errors: ReleaseFormErrors } {
  const errors: ReleaseFormErrors = {}
  const check = <T>(field: keyof ReleaseForm, r: Parsed<T>): T | undefined => {
    if (r.ok) return r.value
    errors[field] = r.error
    return undefined
  }

  const cvName = form.componentVersion.trim()
  if (cvName === '') errors.componentVersion = 'Component version is required.'
  const cvNs = form.componentVersionNamespace.trim()
  const targetNs = check('targetNamespace', optionalName(form.targetNamespace))
  const antiAffinity = check('antiAffinity', parseUserdata(form.antiAffinity)) as
    LabelSelector | null | undefined
  const values = check('values', parseUserdata(form.values))
  const ttl = check('failedJobTTL', parseWholeNumber(form.failedJobTTL))
  const priority = check('priority', parseWholeNumber(form.priority))

  if (Object.keys(errors).length > 0) return { ok: false, errors }

  const uniqueName = form.uniqueName.trim()
  const spec: ReleaseSpec = {
    componentVersionRef: { name: cvName, ...(cvNs ? { namespace: cvNs } : {}) },
    ...(targetNs ? { targetNamespace: targetNs } : {}),
    ...(uniqueName ? { uniqueName } : {}),
    ...(antiAffinity != null ? { antiAffinity } : {}),
    ...(values != null ? { values } : {}),
    ...(ttl !== undefined ? { failedJobTTL: ttl } : {}),
    ...(priority !== undefined ? { priority } : {}),
  }
  return { ok: true, spec }
}

/**
 * Patch that turns `release` into `spec`. uniqueName is immutable and never
 * patched. An optional field missing from `spec` is removed only when the
 * Release currently has it, since removing an absent path fails the patch.
 */
export function buildReleasePatch(release: Release, spec: ReleaseSpec): ReleasePatch {
  const field = <K extends keyof ReleaseSpec>(key: K): ReleaseSpec[K] | null | undefined =>
    spec[key] !== undefined ? spec[key] : release.spec[key] != null ? null : undefined
  // Fields left undefined are skipped by the patch.
  const patch: ReleasePatch['spec'] = {
    componentVersionRef: spec.componentVersionRef,
    targetNamespace: field('targetNamespace'),
    antiAffinity: field('antiAffinity'),
    values: field('values'),
    failedJobTTL: field('failedJobTTL'),
    priority: field('priority'),
  }
  return { spec: patch }
}
