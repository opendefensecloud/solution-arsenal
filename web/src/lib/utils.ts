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

export type TargetHealth =
  | 'healthy'
  | 'degraded'
  | 'failed'
  | 'progressing'
  | 'idle'
  | 'agent-not-reporting'
  | 'unknown'

// targetRollupHealth is the Target's overall health from its conditions; the
// first match wins. ReleasesDeployed and PreflightReady come from the agent's
// report, so a Target without an agent is judged by rendering alone.

export function targetRollupHealth(conditions: Condition[] | undefined): TargetHealth {
  if (!conditions?.length) return 'unknown'
  const rendered = getCondition(conditions, 'ReleasesRendered')
  const bootstrap = getCondition(conditions, 'BootstrapReady')
  // Nothing bound and nothing deployed yet is an empty Target, not a broken one.
  if (!bootstrap && rendered?.status === 'False' && rendered.reason === 'NoReleaseBindings')
    return 'idle'
  if (rendered?.status === 'False' || bootstrap?.status === 'False') return 'degraded'
  if (getCondition(conditions, 'AgentReporting')?.status === 'False') return 'agent-not-reporting'
  const deployed = getCondition(conditions, 'ReleasesDeployed')
  if (deployed?.reason === 'Failed') return 'failed'
  if (deployed?.reason === 'Degraded') return 'degraded'
  if (getCondition(conditions, 'PreflightReady')?.status === 'False') return 'degraded'
  if (deployed?.reason === 'Progressing' || deployed?.reason === 'Pending') return 'progressing'
  const deployedOK = !deployed || deployed.status === 'True' || deployed.reason === 'NoReport'
  if (rendered?.status === 'True' && bootstrap?.status === 'True' && deployedOK) return 'healthy'
  return 'unknown'
}

export function targetHealthLabel(health: TargetHealth): string {
  switch (health) {
    case 'healthy':
      return 'Healthy'
    case 'degraded':
      return 'Degraded'
    case 'failed':
      return 'Failed'
    case 'progressing':
      return 'Progressing'
    case 'idle':
      return 'Idle'
    case 'agent-not-reporting':
      return 'Agent not reporting'
    default:
      return 'Unknown'
  }
}

// targetHealthColor is the StatusDot color of a health value.
export function targetHealthColor(health: TargetHealth): 'success' | 'warning' | 'danger' | 'muted' {
  switch (health) {
    case 'healthy':
      return 'success'
    case 'degraded':
    case 'progressing':
      return 'warning'
    case 'failed':
      return 'danger'
    default:
      return 'muted'
  }
}

// targetHealthVariant is the Badge variant of a health value.
export function targetHealthVariant(
  health: TargetHealth
): 'success' | 'warning' | 'destructive' | 'secondary' {
  switch (targetHealthColor(health)) {
    case 'success':
      return 'success'
    case 'warning':
      return 'warning'
    case 'danger':
      return 'destructive'
    default:
      return 'secondary'
  }
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
