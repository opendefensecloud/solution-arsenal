import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  targetQueries,
  releaseQueries,
  releaseBindingQueries,
  renderTaskQueries,
} from '@/api/queries'
import { useSSE } from '@/hooks/useSSE'
import { useNamespace } from '@/hooks/useNamespace'
import { isForbiddenError } from '@/api/client'
import { ForbiddenAllNs } from '@/components/forbidden-all-ns'
import { StatusDot } from '@/components/ui/status-dot'
import { cn, targetRollupHealth, renderTaskPhase } from '@/lib/utils'

import { GitBranch } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
import { EmptyState } from '@/components/ui/empty-state'

function targetHealthColor(conditions: ReturnType<typeof targetRollupHealth>) {
  switch (conditions) {
    case 'healthy':
      return 'success' as const
    case 'degraded':
      return 'warning' as const
    default:
      return 'muted' as const
  }
}

function phaseColor(phase: ReturnType<typeof renderTaskPhase>) {
  switch (phase) {
    case 'succeeded':
      return 'success' as const
    case 'failed':
      return 'danger' as const
    case 'rendering':
      return 'warning' as const
    default:
      return 'muted' as const
  }
}

function phaseLabel(phase: ReturnType<typeof renderTaskPhase>) {
  switch (phase) {
    case 'succeeded':
      return 'Rendered'
    case 'failed':
      return 'Failed'
    case 'rendering':
      return 'Rendering'
    default:
      return 'Pending'
  }
}

export function PipelinePage() {
  const { namespace } = useNamespace()
  useSSE(namespace)

  const releasesQ = useQuery(releaseQueries.list(namespace))
  const targetsQ = useQuery(targetQueries.list(namespace))
  const bindingsQ = useQuery(releaseBindingQueries.list(namespace))
  const renderTasksQ = useQuery(renderTaskQueries.list(namespace))

  const allReleases = useMemo(() => releasesQ.data?.items ?? [], [releasesQ.data])
  const allTargets = useMemo(() => targetsQ.data?.items ?? [], [targetsQ.data])
  const allBindings = useMemo(() => bindingsQ.data?.items ?? [], [bindingsQ.data])
  const allRenderTasks = useMemo(() => renderTasksQ.data?.items ?? [], [renderTasksQ.data])

  const bindingSet = useMemo(() => {
    const set = new Set<string>()
    for (const b of allBindings) {
      const targetNs = b.spec.targetNamespace || b.metadata.namespace
      set.add(
        `${b.metadata.namespace}/${b.spec.releaseRef.name}/${targetNs}/${b.spec.targetRef.name}`
      )
    }
    return set
  }, [allBindings])

  function relNsAndNameFromRepo(repo: string | undefined): { ns: string; name: string } | null {
    if (!repo) return null
    const parts = repo.split('/')
    const last = parts[parts.length - 1] ?? ''
    if (!last.startsWith('release-')) return null
    const ns = parts[parts.length - 2] ?? ''
    return ns ? { ns, name: last.slice('release-'.length) } : null
  }

  const phaseByCell = useMemo(() => {
    const map = new Map<string, ReturnType<typeof renderTaskPhase>>()
    for (const rt of allRenderTasks) {
      const rel = relNsAndNameFromRepo(rt.spec.repository)
      if (!rel || !rt.spec.ownerName) continue
      map.set(
        `${rt.metadata.namespace}/${rt.spec.ownerName}/${rel.ns}/${rel.name}`,
        renderTaskPhase(rt.status?.conditions)
      )
    }
    return map
  }, [allRenderTasks])

  // Worst-case phase per release across all its render tasks (for row header)
  const phaseByRelease = useMemo(() => {
    const priority: Record<string, number> = { failed: 4, rendering: 3, pending: 2, succeeded: 1 }
    const map = new Map<string, ReturnType<typeof renderTaskPhase>>()
    for (const rt of allRenderTasks) {
      const rel = relNsAndNameFromRepo(rt.spec.repository)
      if (!rel) continue
      const phase = renderTaskPhase(rt.status?.conditions)
      const key = `${rel.ns}/${rel.name}`
      const existing = map.get(key)
      if (!existing || priority[phase] > priority[existing]) map.set(key, phase)
    }
    return map
  }, [allRenderTasks])

  const isLoading =
    releasesQ.isLoading || targetsQ.isLoading || bindingsQ.isLoading || renderTasksQ.isLoading
  const isError = releasesQ.isError || targetsQ.isError || bindingsQ.isError || renderTasksQ.isError

  if (namespace === null && isForbiddenError(releasesQ.error)) {
    return <ForbiddenAllNs resource="releases" />
  }

  if (isError && !isLoading) {
    return <p className="text-destructive">Failed to load pipeline data.</p>
  }

  if (isLoading) return <LoadingState icon={GitBranch} label="Loading pipeline..." />

  const isEmpty = allReleases.length === 0 || allTargets.length === 0

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Pipeline</h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            Release → Target binding matrix &middot; namespace{' '}
            <span className="font-mono">{namespace ?? 'all'}</span>
          </p>
        </div>
        <div className="flex items-center gap-3 text-xs text-muted-foreground">
          <StatusDot color="success" label="Bound" />
          <StatusDot color="warning" label="Rendering" />
          <StatusDot color="danger" label="Failed" />
          <StatusDot color="muted" label="Pending / Unbound" />
        </div>
      </div>

      {isEmpty ? (
        <EmptyState
          icon={GitBranch}
          message={allReleases.length === 0
            ? 'No releases found — create a Release and bind it to a Target to see the pipeline.'
            : 'No targets found — register a Target to see the pipeline.'}
        />
      ) : (
        <div className="overflow-x-auto rounded-lg border border-border">
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/30">
                <th className="sticky left-0 z-10 min-w-[200px] border-r border-border bg-muted/30 px-4 py-3 text-left">
                  <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                    Release
                  </span>
                </th>
                {allTargets.map((target) => {
                  const health = targetRollupHealth(target.status?.conditions)
                  return (
                    <th
                      key={`${target.metadata.namespace}/${target.metadata.name}`}
                      className="min-w-[140px] border-r border-border px-3 py-3 text-left last:border-r-0"
                    >
                      <span className="truncate text-xs font-medium text-foreground">
                        {target.metadata.name}
                      </span>
                      <div className="mt-0.5 flex items-center gap-1">
                        <StatusDot
                          color={targetHealthColor(health)}
                          label={health === 'healthy' ? 'Healthy' : health === 'degraded' ? 'Degraded' : 'Unknown'}
                        />
                      </div>
                      {namespace === null && (
                        <p className="mt-0.5 truncate text-xs font-normal text-muted-foreground">
                          {target.metadata.namespace}
                        </p>
                      )}
                    </th>
                  )
                })}
              </tr>
            </thead>
            <tbody>
              {allReleases.map((release, rowIdx) => {
                const phase =
                  phaseByRelease.get(`${release.metadata.namespace}/${release.metadata.name}`) ?? renderTaskPhase(undefined)

                return (
                  <tr
                    key={`${release.metadata.namespace}/${release.metadata.name}`}
                    className={cn(
                      'border-b border-border last:border-b-0',
                      rowIdx % 2 === 0 ? 'bg-background' : 'bg-muted/10'
                    )}
                  >
                    <td
                      className="sticky left-0 z-10 border-r border-border px-4 py-3"
                      style={{ background: 'inherit' }}
                    >
                      <p className="text-sm font-medium text-foreground">{release.metadata.name}</p>
                      {namespace === null && (
                        <p className="text-xs text-muted-foreground">
                          {release.metadata.namespace}
                        </p>
                      )}
                      <div className="mt-1 flex items-center gap-1">
                        <StatusDot color={phaseColor(phase)} />
                        <span className="text-xs text-muted-foreground">
                          {phaseLabel(phase)}
                        </span>
                      </div>
                    </td>
                    {allTargets.map((target) => {
                      const key = `${release.metadata.namespace}/${release.metadata.name}/${target.metadata.namespace}/${target.metadata.name}`
                      const bound = bindingSet.has(key)
                      const cellPhase =
                        phaseByCell.get(`${target.metadata.namespace}/${target.metadata.name}/${release.metadata.namespace}/${release.metadata.name}`) ??
                        renderTaskPhase(undefined)
                      return (
                        <td
                          key={`${target.metadata.namespace}/${target.metadata.name}`}
                          className="border-r border-border px-3 py-3 last:border-r-0"
                        >
                          <div className="flex items-center justify-center">
                            {bound ? (
                              <StatusDot color={phaseColor(cellPhase)} label={phaseLabel(cellPhase)} />
                            ) : (
                              <span className="text-xs text-muted-foreground/60" aria-label="Not bound">—</span>
                            )}
                          </div>
                        </td>
                      )
                    })}
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      <div className="text-xs text-muted-foreground">
        {allReleases.length} release{allReleases.length !== 1 ? 's' : ''} &middot;{' '}
        {allTargets.length} target{allTargets.length !== 1 ? 's' : ''} &middot; {allBindings.length}{' '}
        binding{allBindings.length !== 1 ? 's' : ''}
      </div>
    </div>
  )
}
