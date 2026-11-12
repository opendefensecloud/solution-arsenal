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
  failed: { label: 'Failed', color: 'danger' },
  rendering: { label: 'Rendering', color: 'warning' },
  pending: { label: 'Pending', color: 'muted' },
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

  return renderPhase ? renderStatus[renderPhase] : undefined
}
