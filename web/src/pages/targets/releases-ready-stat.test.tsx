// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, afterEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import type { Condition } from '@/api/types'
import { ReleasesReadyStat } from './releases-ready-stat'

const releasesDeployed = (status: Condition['status'], reason: string, message: string): Condition[] => [
  { type: 'ReleasesDeployed', status, reason, message, lastTransitionTime: '2026-11-09T11:00:00Z' },
]

afterEach(cleanup)

describe('ReleasesReadyStat', () => {
  it('shows the ReleasesDeployed message', () => {
    render(
      <ReleasesReadyStat
        conditions={releasesDeployed('False', 'Failed', '1 of 3 ready; Failed: monitoring; Progressing: ingress')}
      />
    )
    expect(screen.getByText('1 of 3 ready; Failed: monitoring; Progressing: ingress')).toBeTruthy()
  })

  it('shows a dash without an agent', () => {
    render(<ReleasesReadyStat conditions={[]} />)
    expect(screen.getByText('—')).toBeTruthy()
  })
})
