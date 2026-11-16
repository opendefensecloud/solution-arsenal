// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import {
  conditionLabel,
  targetRollupHealth,
  targetHealthLabel,
  targetHealthColor,
  targetHealthVariant,
} from './utils'
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

  it('is idle when the last binding was removed and the bootstrap is ready', () => {
    expect(
      targetRollupHealth([
        cond('ReleasesRendered', 'False', 'NoReleaseBindings'),
        cond('BootstrapReady', 'True', 'Ready'),
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

  it('is unknown while waiting for the first report', () => {
    expect(
      targetRollupHealth([
        cond('ReleasesRendered', 'True', 'AllRendered'),
        cond('BootstrapReady', 'True', 'Ready'),
        cond('AgentReporting', 'Unknown', 'NoReport'),
        cond('ReleasesDeployed', 'Unknown', 'NoReport'),
      ])
    ).toBe('unknown')
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

    it('is degraded when a binding is unresolved', () => {
      expect(deployed('False', 'Unresolved')).toBe('degraded')
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

    it('is healthy when everything is ready, or no agent is expected', () => {
      expect(deployed('True', 'AllReady')).toBe('healthy')
      expect(targetRollupHealth(healthy)).toBe('healthy')
    })

    it('is unknown until the agent has reported', () => {
      expect(deployed('Unknown', 'NoReport')).toBe('unknown')
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
