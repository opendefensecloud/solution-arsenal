// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import { targetRollupHealth } from './utils'
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
