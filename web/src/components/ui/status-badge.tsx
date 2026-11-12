import type { Condition } from '@/api/types'
import { Badge } from './badge'
import { conditionLabel, getCondition } from '@/lib/utils'

interface StatusBadgeProps {
  conditions?: Condition[]
  type: string
  className?: string
}

const variants = { True: 'success', False: 'destructive', Unknown: 'warning' } as const

export function StatusBadge({ conditions, type, className }: StatusBadgeProps) {
  const condition = getCondition(conditions, type)
  return (
    <Badge
      variant={condition ? variants[condition.status] : 'secondary'}
      title={condition?.message || undefined}
      className={className}
    >
      {conditionLabel(condition)}
    </Badge>
  )
}
