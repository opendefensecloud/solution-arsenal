// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query'
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
  const hasReport = agent !== undefined && agent.reason !== 'NoReport'
  const reportQ = useQuery({ ...targetReportQueries.detail(namespace, name), enabled: hasReport })

  if (!agent) return <span>No agent</span>
  if (!hasReport) return <span>Waiting for first report</span>

  const last = reportQ.data?.spec.lastReportTime
  const reportState = reportQ.isError ? 'error' : reportQ.isSuccess ? 'loaded' : 'loading'
  const reporting = agent.status === 'True'

  let text: string
  if (reporting) text = last ? `Reported ${formatAge(last)} ago` : 'Reporting'
  else text = last ? `Not reporting, last ${formatAge(last)} ago` : 'Not reporting'

  return <span data-report-state={reportState}>{text}</span>
}
