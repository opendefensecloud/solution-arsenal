// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { StatusDot } from '@/components/ui/status-dot'
import { Badge } from '@/components/ui/badge'
import { targetRollupHealth } from '@/lib/utils'
import type { Condition } from '@/api/types'

function healthColor(health: ReturnType<typeof targetRollupHealth>) {
  switch (health) {
    case 'healthy':
      return 'success' as const
    case 'degraded':
      return 'warning' as const
    default:
      return 'muted' as const
  }
}

interface HealthBadgeProps {
  conditions?: Condition[]
}

export function HealthBadge({ conditions }: HealthBadgeProps) {
  const health = targetRollupHealth(conditions)
  const variant = health === 'healthy' ? 'success' : health === 'degraded' ? 'warning' : 'secondary'
  const label = health === 'healthy' ? 'Healthy' : health === 'degraded' ? 'Degraded' : 'Unknown'
  return (
    <div className="flex items-center gap-1.5">
      <StatusDot color={healthColor(health)} />
      <Badge variant={variant as 'success' | 'warning' | 'secondary'}>{label}</Badge>
    </div>
  )
}
