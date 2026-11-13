// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import type { Release } from '@/api/types'
import {
  buildReleasePatch,
  emptyReleaseForm,
  parseWholeNumber,
  parseReleaseForm,
  releaseToForm,
} from './release-form'

const release = {
  metadata: { name: 'r1', namespace: 'default', creationTimestamp: '' },
  spec: {
    componentVersionRef: { name: 'cv1', namespace: 'shared' },
    targetNamespace: 'apps',
    uniqueName: 'kyverno',
    antiAffinity: { matchLabels: { app: 'x' } },
    values: { replicas: 2 },
    failedJobTTL: 600,
    priority: 10,
  },
} as Release

describe('parseWholeNumber', () => {
  it('treats blank input as unset', () => {
    expect(parseWholeNumber('  ')).toEqual({ ok: true, value: undefined })
  })

  it('accepts negative whole numbers', () => {
    expect(parseWholeNumber('-5')).toEqual({ ok: true, value: -5 })
  })

  it('rejects fractions and non-numbers', () => {
    expect(parseWholeNumber('1.5').ok).toBe(false)
    expect(parseWholeNumber('abc').ok).toBe(false)
  })
})

describe('parseReleaseForm', () => {
  it('requires a component version', () => {
    const r = parseReleaseForm(emptyReleaseForm)
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.errors.componentVersion).toMatch(/required/i)
  })

  it('leaves blank optional fields out of the spec', () => {
    expect(parseReleaseForm({ ...emptyReleaseForm, componentVersion: 'cv1' })).toEqual({
      ok: true,
      spec: { componentVersionRef: { name: 'cv1' } },
    })
  })

  it('round-trips every spec field through the form', () => {
    expect(parseReleaseForm(releaseToForm(release))).toEqual({ ok: true, spec: release.spec })
  })

  it('reports per-field errors', () => {
    const r = parseReleaseForm({
      ...emptyReleaseForm,
      componentVersion: 'cv1',
      targetNamespace: 'Bad_NS',
      failedJobTTL: '1.5',
      priority: 'x',
      values: 'a: [',
      antiAffinity: 'a: [',
    })
    expect(r.ok).toBe(false)
    if (!r.ok) {
      expect(Object.keys(r.errors).sort()).toEqual([
        'antiAffinity',
        'failedJobTTL',
        'priority',
        'targetNamespace',
        'values',
      ])
    }
  })
})

describe('buildReleasePatch', () => {
  it('removes cleared fields that the Release currently has', () => {
    expect(buildReleasePatch(release, { componentVersionRef: { name: 'cv2' } })).toEqual({
      spec: {
        componentVersionRef: { name: 'cv2' },
        targetNamespace: null,
        antiAffinity: null,
        values: null,
        failedJobTTL: null,
        priority: null,
      },
    })
  })

  it('does not remove fields the Release does not have', () => {
    const bare = { ...release, spec: { componentVersionRef: { name: 'cv1' }, values: null } }
    expect(buildReleasePatch(bare, { componentVersionRef: { name: 'cv1' } })).toEqual({
      spec: { componentVersionRef: { name: 'cv1' } },
    })
  })

  it('sets changed fields and never patches uniqueName', () => {
    const patch = buildReleasePatch(release, {
      ...release.spec,
      uniqueName: 'other',
      priority: 1,
    })
    expect(patch.spec).not.toHaveProperty('uniqueName')
    expect(patch.spec.priority).toBe(1)
  })
})
