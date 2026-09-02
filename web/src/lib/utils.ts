import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'
import type { Condition } from '@/api/types'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function getCondition(
  conditions: Condition[] | undefined,
  type: string
): Condition | undefined {
  return conditions?.find((c) => c.type === type)
}

export function isReady(conditions: Condition[] | undefined): boolean {
  const ready = getCondition(conditions, 'Ready')
  return ready?.status === 'True'
}

export function formatAge(timestamp: string): string {
  const parsed = new Date(timestamp).getTime()
  if (Number.isNaN(parsed)) return '0s'
  const diff = Math.max(0, Date.now() - parsed)
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  return `${days}d`
}

export function formatDate(timestamp: string): string {
  const parsed = new Date(timestamp).getTime()
  if (Number.isNaN(parsed)) return 'Invalid date'
  return new Date(parsed).toLocaleDateString()
}

export function targetRollupHealth(
  conditions: Condition[] | undefined
): 'healthy' | 'degraded' | 'unknown' {
  if (!conditions?.length) return 'unknown'
  const rendered = getCondition(conditions, 'ReleasesRendered')
  const bootstrap = getCondition(conditions, 'BootstrapReady')
  if (rendered?.status === 'False' || bootstrap?.status === 'False') return 'degraded'
  if (rendered?.status === 'True' && bootstrap?.status === 'True') return 'healthy'
  return 'unknown'
}

export function renderTaskPhase(
  conditions: Condition[] | undefined
): 'pending' | 'rendering' | 'succeeded' | 'failed' {
  if (!conditions?.length) return 'pending'
  if (getCondition(conditions, 'JobFailed')?.status === 'True') return 'failed'
  if (getCondition(conditions, 'JobSucceeded')?.status === 'True') return 'succeeded'
  if (getCondition(conditions, 'JobScheduled')?.status === 'True') return 'rendering'
  return 'pending'
}
