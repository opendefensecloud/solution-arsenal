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
import { Card } from '@/components/ui/card'
import { targetRollupHealth, renderTaskPhase } from '@/lib/utils'
import { Package, Loader, ArrowLeft } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
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
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate({ to: '/releases' })}
            className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="h-4 w-4" />
          </button>
          <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
            <Package className="h-6 w-6 text-primary" />
          </div>
          <div>
            <div className="flex items-center gap-2 flex-wrap">
              <h1 className="text-2xl font-bold text-foreground">{name}</h1>
              <Badge variant="secondary">{namespace}</Badge>
              {cvResolved && (
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
              )}
            </div>
            <p className="text-sm text-muted-foreground font-mono">
              {release.spec.componentVersionRef.name}
            </p>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-6">
        <div className="col-span-2 space-y-6">
          <Card>
            <h3 className="text-sm font-semibold text-foreground mb-3">Properties</h3>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                  Component Version
                </p>
                <p className="mt-1 text-sm text-foreground font-mono">
                  {release.spec.componentVersionRef.name}
                </p>
              </div>
              <div>
                <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                  Namespace
                </p>
                <p className="mt-1 text-sm text-foreground font-mono">{namespace}</p>
              </div>
              <div>
                <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                  Created
                </p>
                <p className="mt-1 text-sm text-foreground">
                  {new Date(release.metadata.creationTimestamp).toLocaleDateString()}
                </p>
              </div>
              {release.status?.effectiveUniqueName && (
                <div>
                  <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    Effective Name
                  </p>
                  <p className="mt-1 text-sm text-foreground font-mono">
                    {release.status.effectiveUniqueName}
                  </p>
                </div>
              )}
              {release.spec.componentVersionNamespace && (
                <div>
                  <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    Version Namespace
                  </p>
                  <p className="mt-1 text-sm text-foreground font-mono">
                    {release.spec.componentVersionNamespace}
                  </p>
                </div>
              )}
            </div>
          </Card>

          <Card>
            <h3 className="text-sm font-semibold text-foreground mb-3">Render Task</h3>
            {renderTasksQ.isLoading ? (
              <p className="text-sm text-muted-foreground">Loading…</p>
            ) : renderTasksQ.isError ? (
              <p className="text-sm text-destructive">Failed to load render task.</p>
            ) : !renderTask ? (
              <p className="text-sm text-muted-foreground">No render task associated.</p>
            ) : (
              <div className="rounded-lg border border-border bg-background p-4">
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
          </Card>
        </div>

        <div className="space-y-3">
          <h3 className="text-sm font-semibold text-foreground">
            Deployed on Targets
            {!bindingsQ.isLoading && !bindingsQ.isError && ` (${boundTargetNames.length})`}
          </h3>
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
