// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { StatusDot } from '@/components/ui/status-dot'
import { Badge } from '@/components/ui/badge'
import {
  targetRollupHealth,
  targetHealthLabel,
  targetHealthColor,
  targetHealthVariant,
} from '@/lib/utils'
import type { Condition } from '@/api/types'

interface HealthBadgeProps {
  conditions?: Condition[]
}

export function HealthBadge({ conditions }: HealthBadgeProps) {
  const health = targetRollupHealth(conditions)
  return (
    <div className="flex items-center gap-1.5">
      <StatusDot color={targetHealthColor(health)} />
      <Badge variant={targetHealthVariant(health)}>{targetHealthLabel(health)}</Badge>
    </div>
  )
}
