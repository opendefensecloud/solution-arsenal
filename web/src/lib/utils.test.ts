// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import { targetRollupHealth, targetHealthLabel } from './utils'
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

  it('flags an agent that stopped reporting, even when otherwise healthy', () => {
    expect(
      targetRollupHealth([
        cond('ReleasesRendered', 'True', 'AllRendered'),
        cond('BootstrapReady', 'True', 'Ready'),
        cond('AgentReporting', 'False', 'Stale'),
      ])
    ).toBe('agent-not-reporting')
  })

  it('stays degraded when rendering fails and the agent is not reporting', () => {
    expect(
      targetRollupHealth([
        cond('ReleasesRendered', 'False', 'RenderFailed'),
        cond('AgentReporting', 'False', 'Stale'),
      ])
    ).toBe('degraded')
  })

  it('is healthy while waiting for the first report', () => {
    expect(
      targetRollupHealth([
        cond('ReleasesRendered', 'True', 'AllRendered'),
        cond('BootstrapReady', 'True', 'Ready'),
        cond('AgentReporting', 'Unknown', 'NoReport'),
      ])
    ).toBe('healthy')
  })
})

describe('targetHealthLabel', () => {
  it('labels every health value', () => {
    expect(targetHealthLabel('healthy')).toBe('Healthy')
    expect(targetHealthLabel('degraded')).toBe('Degraded')
    expect(targetHealthLabel('idle')).toBe('Idle')
    expect(targetHealthLabel('agent-not-reporting')).toBe('Agent not reporting')
    expect(targetHealthLabel('unknown')).toBe('Unknown')
  })
})
