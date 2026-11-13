// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import type { ReleaseBinding, ReleaseDeployment } from '@/api/types'
import { releaseFlags, releaseRowStatus } from './release-status'

const binding = (reason?: string, message = ''): ReleaseBinding => ({
  metadata: { name: 'rb', namespace: 'team-a', creationTimestamp: '2026-11-09T12:00:00Z' },
  spec: { targetRef: { name: 'edge-01' }, releaseRef: { name: 'app' } },
  status: reason
    ? {
        conditions: [
          {
            type: 'Deployed',
            status: reason === 'Filtered' ? 'False' : 'Unknown',
            reason,
            message,
            lastTransitionTime: '2026-11-09T12:00:00Z',
          },
        ],
      }
    : {},
})

describe('releaseRowStatus', () => {
  it('shows a filtered binding as not deployed', () => {
    expect(releaseRowStatus(binding('Filtered', 'lost to team-b/rb'), undefined)).toMatchObject({
      label: 'Not deployed (filtered)',
      color: 'muted',
      detail: 'lost to team-b/rb',
    })
  })

  it('shows an unresolved binding with its message, greyed', () => {
    expect(
      releaseRowStatus(binding('Unresolved', 'Release team-a/app not found'), undefined)
    ).toEqual({
      label: 'Unresolved',
      color: 'warning',
      detail: 'Release team-a/app not found',
      greyed: true,
    })
  })

  it('puts a failed or running render first', () => {
    expect(releaseRowStatus(binding('Unresolved', 'x'), 'failed')).toMatchObject({
      label: 'Render failed',
      color: 'danger',
    })
    expect(releaseRowStatus(binding('Filtered', 'x'), 'rendering')).toMatchObject({
      label: 'Rendering',
      color: 'warning',
    })
  })

  it('falls back to the render phase', () => {
    expect(releaseRowStatus(binding(), 'succeeded')).toEqual({
      label: 'Succeeded',
      color: 'success',
    })
    expect(releaseRowStatus(binding(), 'pending')).toEqual({ label: 'Pending', color: 'muted' })
    expect(releaseRowStatus(undefined, 'succeeded')).toEqual({
      label: 'Succeeded',
      color: 'success',
    })
  })

  it('shows nothing without a render or a Deployed condition', () => {
    expect(releaseRowStatus(binding(), undefined)).toBeUndefined()
    expect(releaseRowStatus(undefined, undefined)).toBeUndefined()
  })

  it('puts a failed or running render before a deployed release', () => {
    const ready = deployed('True', 'Ready')
    expect(releaseRowStatus(ready, 'failed')).toMatchObject({
      label: 'Render failed',
      color: 'danger',
    })
    expect(releaseRowStatus(ready, 'rendering')).toMatchObject({
      label: 'Rendering',
      color: 'warning',
    })
  })

  it('shows a deployed release as ready', () => {
    expect(releaseRowStatus(deployed('True', 'Ready'), 'succeeded')).toEqual({
      label: 'Ready',
      color: 'success',
    })
  })

  it('shows a failed release with its message', () => {
    expect(releaseRowStatus(deployed('False', 'Failed', 'HelmRelease: boom'), 'succeeded')).toEqual(
      { label: 'Failed', color: 'danger', detail: 'HelmRelease: boom' }
    )
  })

  it('colours degraded and progressing as warnings, pending as muted', () => {
    expect(releaseRowStatus(deployed('False', 'Degraded', 'x'), 'succeeded')).toMatchObject({
      label: 'Degraded',
      color: 'warning',
    })
    expect(releaseRowStatus(deployed('False', 'Progressing', 'x'), 'succeeded')).toMatchObject({
      label: 'Progressing',
      color: 'warning',
    })
    expect(releaseRowStatus(deployed('False', 'Pending', ''), 'succeeded')).toEqual({
      label: 'Pending',
      color: 'muted',
    })
  })

  it('shows the last seen state of a silent agent, greyed', () => {
    expect(
      releaseRowStatus(
        deployed('Unknown', 'AgentNotReporting', 'silent', { phase: 'Failed' }),
        'succeeded'
      )
    ).toMatchObject({ label: 'Unknown (last seen Failed)', color: 'muted', greyed: true })
    expect(
      releaseRowStatus(deployed('Unknown', 'ReportIncomplete', 'x'), 'succeeded')
    ).toMatchObject({ label: 'Unknown', greyed: true })
  })

  it('shows the render phase before the first report', () => {
    expect(releaseRowStatus(deployed('Unknown', 'NoReport', 'x'), 'succeeded')).toEqual({
      label: 'Succeeded',
      color: 'success',
    })
  })

  it('adds no detail of its own when the controller wrote no message', () => {
    expect(
      releaseRowStatus(
        deployed('False', 'Progressing', '', {
          liveRevision: '0.0.2-bb+0cc9a8446c95',
          desiredRevision: 'v0.0.2-bb',
        }),
        'succeeded'
      )
    ).toEqual({ label: 'Progressing', color: 'warning' })
  })
})

describe('releaseFlags', () => {
  it('labels the flags that are true', () => {
    const b = deployed('True', 'Ready')
    b.status!.conditions!.push(
      flag('RolledBack', 'True', 'Rolled back to 0.0.1-aa'),
      flag('Drifted', 'True'),
      flag('TestFailed', 'False'),
      flag('Verified', 'True')
    )
    expect(releaseFlags(b)).toEqual([
      { label: 'Rolled back', detail: 'Rolled back to 0.0.1-aa' },
      { label: 'Drifted', detail: '' },
    ])
  })

  it('has none without a binding', () => {
    expect(releaseFlags(undefined)).toEqual([])
  })
})

function deployed(
  status: 'True' | 'False' | 'Unknown',
  reason: string,
  message = '',
  deployment?: ReleaseDeployment
): ReleaseBinding {
  const b = binding()
  b.status = { conditions: [{ ...flag('Deployed', status, message), reason }], deployment }
  return b
}

function flag(type: string, status: 'True' | 'False' | 'Unknown', message = '') {
  return { type, status, reason: type, message, lastTransitionTime: '2026-11-09T12:00:00Z' }
}
