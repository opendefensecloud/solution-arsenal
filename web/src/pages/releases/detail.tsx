// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo } from 'react'
import { useParams, useNavigate, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import {
  releaseQueries,
  releaseBindingQueries,
  targetQueries,
  renderTaskQueries,
} from '@/api/queries'
import { StatusDot } from '@/components/ui/status-dot'
import { Badge } from '@/components/ui/badge'
import { targetRollupHealth, renderTaskPhase } from '@/lib/utils'
import { Package, Loader } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
import { BackButton } from '@/components/ui/back-button'
import type { Condition, Target } from '@/api/types'

function phaseColor(p: ReturnType<typeof renderTaskPhase>) {
  return p === 'succeeded'
    ? ('success' as const)
    : p === 'failed'
      ? ('danger' as const)
      : p === 'rendering'
        ? ('warning' as const)
        : ('muted' as const)
}

function healthColor(h: ReturnType<typeof targetRollupHealth>) {
  return h === 'healthy'
    ? ('success' as const)
    : h === 'degraded'
      ? ('warning' as const)
      : ('muted' as const)
}

function conditionStatus(conditions: Condition[] | undefined, type: string) {
  return conditions?.find((c) => c.type === type)
}

export function ReleaseDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const navigate = useNavigate()

  const releaseQ = useQuery(releaseQueries.detail(namespace, name))
  const bindingsQ = useQuery(releaseBindingQueries.list(namespace))
  const targetsQ = useQuery(targetQueries.list(null))
  const renderTasksQ = useQuery(renderTaskQueries.list(namespace))

  const release = releaseQ.data

  const renderTask = useMemo(
    () =>
      renderTasksQ.data?.items.find((rt) => {
        const last = (rt.spec.repository ?? '').split('/').pop() ?? ''
        return last === `release-${name}`
      }),
    [renderTasksQ.data, name]
  )

  const phase = renderTaskPhase(renderTask?.status?.conditions)

  const boundTargetNames = useMemo(
    () =>
      (bindingsQ.data?.items ?? [])
        .filter((b) => b.spec.releaseRef.name === name)
        .map((b) => ({
          name: b.spec.targetRef.name,
          namespace: b.spec.targetNamespace ?? b.metadata.namespace,
        })),
    [bindingsQ.data, name]
  )

  const targetMap = useMemo(() => {
    const m = new Map<string, Target>()
    for (const t of targetsQ.data?.items ?? [])
      m.set(`${t.metadata.namespace}/${t.metadata.name}`, t)
    return m
  }, [targetsQ.data])

  const cvResolved = conditionStatus(release?.status?.conditions, 'ComponentVersionResolved')

  if (releaseQ.isLoading) return <LoadingState icon={Package} label="Loading…" />

  if (releaseQ.isError) {
    return <p className="text-destructive">Failed to load release.</p>
  }

  if (!release) {
    return <p className="text-destructive">Release not found.</p>
  }

  return (
    <div className="space-y-6">
      <BackButton label="Back to Releases" onClick={() => navigate({ to: '/releases' })} />

      <div className="flex items-start gap-4">
        <div className="rounded-xl bg-primary/10 p-3">
          <Package className="h-6 w-6 text-primary" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h1 className="text-2xl font-bold text-foreground">{name}</h1>
            <Badge variant="secondary">{namespace}</Badge>
            {cvResolved && (
              <div className="flex items-center gap-1.5">
                <StatusDot
                  color={
                    cvResolved.status === 'True'
                      ? 'success'
                      : cvResolved.status === 'False'
                        ? 'danger'
                        : 'muted'
                  }
                />
                <Badge
                  variant={
                    cvResolved.status === 'True'
                      ? 'success'
                      : cvResolved.status === 'False'
                        ? 'destructive'
                        : 'secondary'
                  }
                >
                  {cvResolved.status === 'True'
                    ? 'Version Resolved'
                    : cvResolved.status === 'False'
                      ? 'Unresolved'
                      : 'Resolving'}
                </Badge>
              </div>
            )}
          </div>
          <p className="mt-1 text-sm text-muted-foreground font-mono">
            {release.spec.componentVersionRef.name}
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            {[
              { label: 'Component Version', value: release.spec.componentVersionRef.name },
              { label: 'Namespace', value: namespace },
              {
                label: 'Created',
                value: new Date(release.metadata.creationTimestamp).toLocaleDateString(),
              },
              ...(release.status?.effectiveUniqueName
                ? [{ label: 'Effective Name', value: release.status.effectiveUniqueName }]
                : []),
            ].map(({ label, value }) => (
              <div key={label} className="rounded-lg border border-border bg-card p-3">
                <p className="text-xs font-medium text-muted-foreground">{label}</p>
                <p className="mt-0.5 text-sm font-semibold text-foreground font-mono">{value}</p>
              </div>
            ))}
          </div>

          <div>
            <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
              Render Task
            </h2>
            {renderTasksQ.isLoading ? (
              <p className="text-sm text-muted-foreground">Loading…</p>
            ) : renderTasksQ.isError ? (
              <p className="text-sm text-destructive">Failed to load render task.</p>
            ) : !renderTask ? (
              <p className="text-sm text-muted-foreground">No render task associated.</p>
            ) : (
              <div className="rounded-lg border border-border bg-card p-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Loader
                      className={`h-4 w-4 text-muted-foreground ${phase === 'rendering' ? 'animate-spin' : ''}`}
                    />
                    <span className="text-sm font-medium text-foreground font-mono">
                      {renderTask.metadata.name}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <StatusDot color={phaseColor(phase)} />
                    <span className="text-xs capitalize text-muted-foreground">{phase}</span>
                  </div>
                </div>
                {renderTask.status?.chartURL && (
                  <p className="mt-2 text-xs text-muted-foreground font-mono">
                    Chart: {renderTask.status.chartURL}
                  </p>
                )}
              </div>
            )}
          </div>
        </div>

        <div className="space-y-3">
          <h2 className="text-sm font-semibold text-foreground">
            Deployed on Targets{!bindingsQ.isLoading && !bindingsQ.isError && ` (${boundTargetNames.length})`}
          </h2>
          {bindingsQ.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading…</p>
          ) : bindingsQ.isError ? (
            <p className="text-sm text-destructive">Failed to load deployment targets.</p>
          ) : boundTargetNames.length === 0 ? (
            <p className="text-sm italic text-muted-foreground">Not deployed to any targets.</p>
          ) : (
            boundTargetNames.map(({ name: tName, namespace: tNs }) => {
              const target = targetMap.get(`${tNs}/${tName}`)
              const healthKnown = !targetsQ.isLoading && !targetsQ.isError
              const health = healthKnown ? targetRollupHealth(target?.status?.conditions) : null
              return (
                <div
                  key={`${tNs}/${tName}`}
                  className="rounded-lg border border-border bg-card p-3"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      {health !== null && <StatusDot color={healthColor(health)} />}
                      <Link
                        to="/targets/$namespace/$name"
                        params={{ namespace: tNs, name: tName }}
                        className="text-sm font-medium text-foreground hover:text-primary transition-colors"
                      >
                        {tName}
                      </Link>
                    </div>
                    {health !== null && (
                      <span className="text-xs capitalize text-muted-foreground">{health}</span>
                    )}
                  </div>
                  {tNs !== namespace && (
                    <p className="mt-1 text-xs text-muted-foreground font-mono">{tNs}</p>
                  )}
                </div>
              )
            })
          )}
        </div>
      </div>
    </div>
  )
}
