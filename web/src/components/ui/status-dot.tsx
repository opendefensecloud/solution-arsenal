import { cn } from '@/lib/utils'

const colorClass = {
  success: 'bg-green-500',
  warning: 'bg-amber-500',
  danger: 'bg-red-500',
  muted: 'bg-muted-foreground/40',
} as const

export function StatusDot({ color, label }: { color: keyof typeof colorClass; label?: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <span
        className={cn('h-2 w-2 rounded-full shrink-0', colorClass[color])}
        role={label ? undefined : 'img'}
        aria-label={label ? undefined : color}
        aria-hidden={label ? true : undefined}
        title={label ?? color}
      />
      {label && <span className="text-xs text-muted-foreground">{label}</span>}
    </span>
  )
}
