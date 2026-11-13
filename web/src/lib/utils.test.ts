// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import { targetRollupHealth, targetHealthLabel, targetHealthColor, targetHealthVariant } from './utils'
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

  describe('with ReleasesDeployed', () => {
    const healthy = [cond('ReleasesRendered', 'True', 'AllRendered'), cond('BootstrapReady', 'True', 'Ready')]
    const deployed = (status: Condition['status'], reason: string, ...more: Condition[]) =>
      targetRollupHealth([...healthy, cond('ReleasesDeployed', status, reason), ...more])

    it('is failed when a release failed', () => {
      expect(deployed('False', 'Failed')).toBe('failed')
    })

    it('is degraded when a release is degraded', () => {
      expect(deployed('False', 'Degraded')).toBe('degraded')
    })

    it('is degraded when preflight failed', () => {
      expect(
        deployed('True', 'AllReady', cond('PreflightReady', 'False', 'ChecksFailed'))
      ).toBe('degraded')
    })

    it('ignores preflight that could not be evaluated', () => {
      expect(
        deployed('True', 'AllReady', cond('PreflightReady', 'Unknown', 'ChecksErrored'))
      ).toBe('healthy')
    })

    it('is progressing while a release rolls out or waits', () => {
      expect(deployed('False', 'Progressing')).toBe('progressing')
      expect(deployed('False', 'Pending')).toBe('progressing')
    })

    it('is healthy when everything is ready, absent, or the agent has not reported yet', () => {
      expect(deployed('True', 'AllReady')).toBe('healthy')
      expect(deployed('Unknown', 'NoReport')).toBe('healthy')
      expect(targetRollupHealth(healthy)).toBe('healthy')
    })

    it('is unknown when the report is incomplete', () => {
      expect(deployed('Unknown', 'ReportIncomplete')).toBe('unknown')
    })

    it('puts an agent that stopped reporting before a failed release', () => {
      expect(deployed('False', 'Failed', cond('AgentReporting', 'False', 'Stale'))).toBe(
        'agent-not-reporting'
      )
    })
  })
})

describe('targetHealthLabel', () => {
  it('labels every health value', () => {
    expect(targetHealthLabel('healthy')).toBe('Healthy')
    expect(targetHealthLabel('degraded')).toBe('Degraded')
    expect(targetHealthLabel('idle')).toBe('Idle')
    expect(targetHealthLabel('agent-not-reporting')).toBe('Agent not reporting')
    expect(targetHealthLabel('unknown')).toBe('Unknown')
    expect(targetHealthLabel('failed')).toBe('Failed')
    expect(targetHealthLabel('progressing')).toBe('Progressing')
  })
})

describe('targetHealthColor and targetHealthVariant', () => {
  it('shows failed as danger and progressing as warning', () => {
    expect(targetHealthColor('failed')).toBe('danger')
    expect(targetHealthVariant('failed')).toBe('destructive')
    expect(targetHealthColor('progressing')).toBe('warning')
    expect(targetHealthVariant('progressing')).toBe('warning')
  })

  it('keeps the existing colors', () => {
    expect(targetHealthColor('healthy')).toBe('success')
    expect(targetHealthVariant('healthy')).toBe('success')
    expect(targetHealthColor('degraded')).toBe('warning')
    expect(targetHealthVariant('degraded')).toBe('warning')
    expect(targetHealthColor('unknown')).toBe('muted')
    expect(targetHealthVariant('agent-not-reporting')).toBe('secondary')
  })
})
