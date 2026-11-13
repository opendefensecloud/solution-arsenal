// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo, useState } from 'react'
import { useParams, useNavigate, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import {
  releaseQueries,
  releaseBindingQueries,
  targetQueries,
  renderTaskQueries,
} from '@/api/queries'
import { StatusDot } from '@/components/ui/status-dot'
import { StatusBadge } from '@/components/ui/status-badge'
import { Card } from '@/components/ui/card'
import { targetRollupHealth, renderTaskPhase } from '@/lib/utils'
import { Package, Loader } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
import { DetailHeader } from '@/components/ui/detail-header'
import { DetailSection } from '@/components/ui/detail-section'
import { ConditionsTable } from '@/components/ui/conditions-table'
import { YamlBlock } from '@/components/ui/yaml-block'
import type { Condition, Target } from '@/api/types'
import { EditReleaseDialog } from './edit-release-dialog'
import { DeleteReleaseDialog } from './delete-release-dialog'

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

function hasData(v: unknown) {
  return v != null && (typeof v !== 'object' || Object.keys(v as object).length > 0)
}

export function ReleaseDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const navigate = useNavigate()
  const [showEdit, setShowEdit] = useState(false)
  const [showDelete, setShowDelete] = useState(false)

  const releaseQ = useQuery(releaseQueries.detail(namespace, name))
  const bindingsQ = useQuery(releaseBindingQueries.list(namespace, name))
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
      (bindingsQ.data?.items ?? []).map((b) => ({
        name: b.spec.targetRef.name,
        namespace: b.spec.targetRef.namespace ?? b.metadata.namespace,
      })),
    [bindingsQ.data, name]
  )

  const targetMap = useMemo(() => {
    const m = new Map<string, Target>()
    for (const t of targetsQ.data?.items ?? [])
      m.set(`${t.metadata.namespace}/${t.metadata.name}`, t)
    return m
  }, [targetsQ.data])

  if (releaseQ.isLoading) return <LoadingState icon={Package} label="Loading…" />

  if (releaseQ.isError) {
    return <p className="text-destructive">Failed to load release.</p>
  }

  if (!release) {
    return <p className="text-destructive">Release not found.</p>
  }

  return (
    <div className="space-y-6">
      <DetailHeader
        icon={Package}
        title={name}
        namespace={namespace}
        subtitle={release.spec.componentVersionRef.name}
        badges={<StatusBadge conditions={release.status?.conditions} type="Renderable" />}
        actions={
          <>
            <button
              type="button"
              onClick={() => setShowEdit(true)}
              className="rounded-md border border-border px-3 py-1.5 text-sm font-medium text-foreground hover:bg-accent"
            >
              Edit
            </button>
            <button
              type="button"
              onClick={() => setShowDelete(true)}
              className="rounded-md border border-destructive/40 px-3 py-1.5 text-sm font-medium text-destructive hover:bg-destructive/10"
            >
              Delete
            </button>
          </>
        }
        backLabel="Back to Releases"
        onBack={() => navigate({ to: '/releases' })}
      />

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
              {release.spec.componentVersionRef?.namespace && (
                <div>
                  <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    Version Namespace
                  </p>
                  <p className="mt-1 text-sm text-foreground font-mono">
                    {release.spec.componentVersionRef.namespace}
                  </p>
                </div>
              )}
              {release.spec.targetNamespace && (
                <div>
                  <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    Target Namespace
                  </p>
                  <p className="mt-1 text-sm text-foreground font-mono">
                    {release.spec.targetNamespace}
                  </p>
                </div>
              )}
              <div>
                <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                  Priority
                </p>
                <p className="mt-1 text-sm text-foreground font-mono">
                  {release.spec.priority ?? 0}
                </p>
              </div>
              {release.spec.failedJobTTL != null && (
                <div>
                  <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
                    Failed Job TTL
                  </p>
                  <p className="mt-1 text-sm text-foreground font-mono">
                    {release.spec.failedJobTTL}s
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
                    <Link
                      to="/rendertasks/$namespace/$name"
                      params={{ namespace, name: renderTask.metadata.name }}
                      className="text-sm font-medium text-foreground font-mono hover:text-primary transition-colors"
                    >
                      {renderTask.metadata.name}
                    </Link>
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

          {hasData(release.spec.antiAffinity) && (
            <DetailSection title="Anti-Affinity">
              <YamlBlock value={release.spec.antiAffinity} />
            </DetailSection>
          )}

          {hasData(release.spec.values) && (
            <DetailSection title="Values">
              <YamlBlock value={release.spec.values} />
            </DetailSection>
          )}

          <DetailSection title="Conditions">
            <ConditionsTable conditions={release.status?.conditions} />
          </DetailSection>
        </div>

        <DetailSection
          title="Deployed on Targets"
          count={!bindingsQ.isLoading && !bindingsQ.isError ? boundTargetNames.length : undefined}
        >
          {bindingsQ.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading…</p>
          ) : bindingsQ.isError ? (
            <p className="text-sm text-destructive">Failed to load deployment targets.</p>
          ) : boundTargetNames.length === 0 ? (
            <p className="text-sm italic text-muted-foreground">Not deployed to any targets.</p>
          ) : (
            <div className="space-y-3">
              {boundTargetNames.map(({ name: tName, namespace: tNs }) => {
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
              })}
            </div>
          )}
        </DetailSection>
      </div>

      {showEdit && <EditReleaseDialog open onOpenChange={setShowEdit} release={release} />}
      <DeleteReleaseDialog
        open={showDelete}
        onOpenChange={setShowDelete}
        namespace={namespace}
        name={name}
      />
    </div>
  )
}
