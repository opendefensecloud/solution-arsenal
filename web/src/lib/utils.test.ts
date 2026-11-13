// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import { conditionLabel, targetRollupHealth } from './utils'
import type { Condition } from '@/api/types'

const cond = (type: string, status: Condition['status'], reason: string): Condition => ({
  type,
  status,
  reason,
  message: '',
  lastTransitionTime: '2026-01-01T00:00:00Z',
})

describe('targetRollupHealth', () => {
  it('is idle when nothing is bound and no bootstrap exists', () => {
    expect(
      targetRollupHealth([
        cond('RegistryResolved', 'True', 'Resolved'),
        cond('ReleasesRendered', 'False', 'NoReleaseBindings'),
        cond('ReleasesResolved', 'False', 'NoReleaseBindings'),
      ])
    ).toBe('idle')
  })

  it('stays degraded when a leftover bootstrap is failing', () => {
    expect(
      targetRollupHealth([
        cond('ReleasesRendered', 'False', 'NoReleaseBindings'),
        cond('BootstrapReady', 'False', 'Failed'),
      ])
    ).toBe('degraded')
  })

  it('is degraded when rendering fails for another reason', () => {
    expect(
      targetRollupHealth([cond('ReleasesRendered', 'False', 'AllReleaseBindingsFiltered')])
    ).toBe('degraded')
  })

  it('is healthy when releases and bootstrap are ready', () => {
    expect(
      targetRollupHealth([
        cond('ReleasesRendered', 'True', 'AllRendered'),
        cond('BootstrapReady', 'True', 'Ready'),
      ])
    ).toBe('healthy')
  })

  it('is unknown without conditions', () => {
    expect(targetRollupHealth(undefined)).toBe('unknown')
  })
})

describe('conditionLabel', () => {
  it('names a holding condition by its type, not its reason', () => {
    expect(conditionLabel(cond('Renderable', 'True', 'DescriptorAvailable'))).toBe('Renderable')
  })

  it('shows why a condition does not hold', () => {
    expect(conditionLabel(cond('Renderable', 'False', 'NotFound'))).toBe('NotFound')
  })

  it('falls back when the reason is empty or the condition is missing', () => {
    expect(conditionLabel(cond('Renderable', 'False', ''))).toBe('Not Renderable')
    expect(conditionLabel(cond('Renderable', 'Unknown', ''))).toBe('Pending')
    expect(conditionLabel(undefined)).toBe('Unknown')
  })
})
