// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import { parseK8sName, K8S_NAME_MAX_LENGTH } from './k8s-name'

describe('parseK8sName', () => {
  it('accepts a valid name and returns it trimmed', () => {
    expect(parseK8sName('my-target-1')).toEqual({ ok: true, value: 'my-target-1' })
  })

  it('trims surrounding whitespace before validating and returning', () => {
    expect(parseK8sName('  my-target  ')).toEqual({ ok: true, value: 'my-target' })
  })

  it('rejects an empty string', () => {
    expect(parseK8sName('')).toEqual({ ok: false, error: 'Name is required.' })
  })

  it('rejects a whitespace-only string', () => {
    expect(parseK8sName('   ')).toEqual({ ok: false, error: 'Name is required.' })
  })

  it('uses a custom required message when provided', () => {
    expect(parseK8sName('', 'Release is required.')).toEqual({
      ok: false,
      error: 'Release is required.',
    })
    expect(parseK8sName('   ', 'Render registry is required.')).toEqual({
      ok: false,
      error: 'Render registry is required.',
    })
  })

  it('still returns the format error (not the required message) for invalid non-empty input', () => {
    expect(parseK8sName('bad_name', 'Release is required.')).toEqual({
      ok: false,
      error: 'Label must be ≤63 and contain only lowercase alphanumeric characters and "-".',
    })
  })

  it('rejects invalid names', () => {
    expect(parseK8sName('My-Target').ok).toBe(false)
    expect(parseK8sName('my_target').ok).toBe(false)
    expect(parseK8sName('-my-target').ok).toBe(false)
    expect(parseK8sName('my-target-').ok).toBe(false)
    expect(parseK8sName('my/target').ok).toBe(false)
    expect(parseK8sName('my.target').ok).toBe(false)
  })

  it('accepts a name of exactly the maximum length', () => {
    const name = 'a'.repeat(K8S_NAME_MAX_LENGTH)
    expect(parseK8sName(name)).toEqual({ ok: true, value: name })
    expect(parseK8sName(`  ${name}  `)).toEqual({ ok: true, value: name })
  })

  it('rejects a trimmed name longer than the maximum', () => {
    expect(parseK8sName('a'.repeat(K8S_NAME_MAX_LENGTH + 1)).ok).toBe(false)
    expect(parseK8sName(`  ${'a'.repeat(K8S_NAME_MAX_LENGTH + 1)}  `).ok).toBe(false)
  })
})
