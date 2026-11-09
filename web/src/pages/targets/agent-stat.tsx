// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query'
import { isForbiddenError } from '@/api/client'
import { targetReportQueries } from '@/api/queries'
import type { Condition } from '@/api/types'
import { formatAge, getCondition } from '@/lib/utils'

interface AgentStatProps {
  namespace: string
  name: string
  conditions?: Condition[]
}

// AgentStat is the Target page's Agent card value. It reads the agent's state
// from the AgentReporting condition, and the time of the last report from the
// TargetReport. If the report can't be read, for example because the viewer
// may not, the condition alone decides.
export function AgentStat({ namespace, name, conditions }: AgentStatProps) {
  const agent = getCondition(conditions, 'AgentReporting')
  // A report from a clock running ahead has no meaningful age to show.
  const hasReport =
    agent !== undefined && agent.reason !== 'NoReport' && agent.reason !== 'ClockSkew'
  const reportQ = useQuery({
    ...targetReportQueries.detail(namespace, name),
    enabled: hasReport,
    // A viewer without read access won't gain it by asking again.
    retry: (failures, err) => !isForbiddenError(err) && failures < 3,
  })

  if (!agent) return <span>No agent</span>
  if (agent.reason === 'ClockSkew') return <span title={agent.message}>Agent clock ahead</span>
  if (!hasReport) return <span>Waiting for first report</span>

  const last = reportQ.data?.spec.lastReportTime
  const reportState = reportQ.isError ? 'error' : reportQ.isSuccess ? 'loaded' : 'loading'
  const reporting = agent.status === 'True'
  const title = isForbiddenError(reportQ.error)
    ? 'Time of the last report hidden: no read access to this TargetReport'
    : undefined

  let text: string
  if (reporting) text = last ? `Reported ${formatAge(last)} ago` : 'Reporting'
  else text = last ? `Not reporting, last ${formatAge(last)} ago` : 'Not reporting'

  return (
    <span data-report-state={reportState} title={title}>
      {text}
    </span>
  )
}
