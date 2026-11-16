// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { ReleaseBinding } from '@/api/types'
import { getCondition, type renderTaskPhase } from '@/lib/utils'

export type RenderPhase = ReturnType<typeof renderTaskPhase>

export type ReleaseRowStatus = {
  label: string
  color: 'success' | 'warning' | 'danger' | 'muted'
  detail?: string
  greyed?: boolean
}

const renderStatus: Record<RenderPhase, ReleaseRowStatus> = {
  succeeded: { label: 'Succeeded', color: 'success' },
  failed: { label: 'Render failed', color: 'danger' },
  rendering: { label: 'Rendering', color: 'warning' },
  pending: { label: 'Pending', color: 'muted' },
}

const deployedColor: Record<string, ReleaseRowStatus['color']> = {
  Failed: 'danger',
  Degraded: 'warning',
  Progressing: 'warning',
  Pending: 'muted',
}

// releaseRowStatus is the one status a Bound Releases row shows: a failed or
// running render first, then the binding's Deployed condition, then the render
// phase. undefined when there is nothing to show yet.
export function releaseRowStatus(
  binding: ReleaseBinding | undefined,
  renderPhase: RenderPhase | undefined
): ReleaseRowStatus | undefined {
  if (renderPhase === 'failed' || renderPhase === 'rendering') return renderStatus[renderPhase]

  const deployed = getCondition(binding?.status?.conditions, 'Deployed')
  if (deployed?.reason === 'Filtered')
    return { label: 'Not deployed (filtered)', color: 'muted', detail: deployed.message }
  if (deployed?.reason === 'Unresolved')
    return { label: 'Unresolved', color: 'warning', detail: deployed.message, greyed: true }

  // Before the first report, the render is all there is to say.
  if (!deployed || deployed.reason === 'NoReport')
    return renderPhase ? renderStatus[renderPhase] : undefined

  if (deployed.status === 'True') return { label: 'Ready', color: 'success' }

  const deployment = binding?.status?.deployment
  if (deployed.status === 'Unknown') {
    const last = deployment?.phase
    return {
      label: last ? `Unknown (last seen ${last})` : 'Unknown',
      color: 'muted',
      detail: deployed.message || undefined,
      greyed: true,
    }
  }

  const status: ReleaseRowStatus = {
    label: deployed.reason,
    color: deployedColor[deployed.reason] ?? 'warning',
  }
  if (deployed.message) status.detail = deployed.message
  return status
}

export type ReleaseFlag = { label: string; detail: string }

// The binding conditions shown as small labels on a row while True.
const flagLabels: [type: string, label: string][] = [
  ['RolledBack', 'Rolled back'],
  ['Drifted', 'Drifted'],
  ['TestFailed', 'Test failed'],
]

// releaseFlags lists the flag conditions a row labels, with Flux's message,
// which names the raw revisions of a rollback.
export function releaseFlags(binding: ReleaseBinding | undefined): ReleaseFlag[] {
  const conditions = binding?.status?.conditions
  return flagLabels.flatMap(([type, label]) => {
    const cond = getCondition(conditions, type)
    return cond?.status === 'True' ? [{ label, detail: cond.message }] : []
  })
}
