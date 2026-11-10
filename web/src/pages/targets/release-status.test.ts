// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import type { ReleaseBinding } from '@/api/types'
import { releaseRowStatus } from './release-status'

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
      label: 'Failed',
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
})
