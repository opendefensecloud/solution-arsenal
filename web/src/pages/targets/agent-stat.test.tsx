// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { Condition } from '@/api/types'
import { AgentStat } from './agent-stat'

const { reportFn } = vi.hoisted(() => ({ reportFn: vi.fn() }))
vi.mock('@/api/queries', () => ({
  targetReportQueries: {
    detail: (namespace: string, name: string) => ({
      queryKey: ['targetreports', namespace, name],
      queryFn: reportFn,
    }),
  },
}))

const NOW = new Date('2026-11-09T12:00:40Z')

const agentReporting = (status: Condition['status'], reason: string): Condition[] => [
  {
    type: 'AgentReporting',
    status,
    reason,
    message: '',
    lastTransitionTime: '2026-11-09T11:00:00Z',
  },
]

function renderStat(
  conditions: Condition[] | undefined,
  qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
) {
  return render(
    <QueryClientProvider client={qc}>
      <AgentStat namespace="team-a" name="edge-01" conditions={conditions} />
    </QueryClientProvider>
  )
}

describe('AgentStat', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(NOW)
    reportFn.mockResolvedValue({ spec: { lastReportTime: '2026-11-09T12:00:00Z' } })
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
    reportFn.mockReset()
  })

  it('shows when the agent last reported', async () => {
    renderStat(agentReporting('True', 'Reporting'))
    expect(await screen.findByText('Reported 40s ago')).toBeInTheDocument()
  })

  it('waits for the first report', () => {
    renderStat(agentReporting('Unknown', 'NoReport'))
    expect(screen.getByText('Waiting for first report')).toBeInTheDocument()
  })

  it('shows no agent without the condition', () => {
    renderStat([])
    expect(screen.getByText('No agent')).toBeInTheDocument()
    expect(reportFn).not.toHaveBeenCalled()
  })

  it('falls back to the condition when the report is forbidden', async () => {
    reportFn.mockRejectedValue({ status: 403, message: 'forbidden' })
    const { container } = renderStat(agentReporting('True', 'Reporting'))
    await vi.waitFor(() =>
      expect(container.querySelector('[data-report-state="error"]')).not.toBeNull()
    )
    expect(screen.getByText('Reporting')).toBeInTheDocument()
  })

  it('does not retry a forbidden report and says why the time is missing', async () => {
    reportFn.mockRejectedValue({ status: 403, message: 'forbidden' })
    // The app's default client retries failed queries.
    const { container } = renderStat(agentReporting('True', 'Reporting'), new QueryClient())
    await vi.waitFor(() =>
      expect(container.querySelector('[data-report-state="error"]')).not.toBeNull()
    )
    expect(reportFn).toHaveBeenCalledTimes(1)
    expect(screen.getByText('Reporting')).toHaveAttribute(
      'title',
      expect.stringContaining('no read access')
    )
  })

  it('shows a stale agent with its last report', async () => {
    reportFn.mockResolvedValue({ spec: { lastReportTime: '2026-11-09T11:45:40Z' } })
    renderStat(agentReporting('False', 'Stale'))
    expect(await screen.findByText('Not reporting, last 15m ago')).toBeInTheDocument()
  })
})
