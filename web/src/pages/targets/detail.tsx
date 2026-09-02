// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo, useState } from 'react'
import { useParams, useNavigate, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import {
  targetQueries,
  releaseBindingQueries,
  renderTaskQueries,
  registryBindingQueries,
} from '@/api/queries'
import { StatusDot } from '@/components/ui/status-dot'
import { Badge } from '@/components/ui/badge'
import { cn, targetRollupHealth, renderTaskPhase } from '@/lib/utils'
import { Server, Package } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
import type { RenderTask } from '@/api/types'
import { DeleteTargetDialog } from './delete-target-dialog'
import { EditTargetDialog } from './edit-target-dialog'
import { DetailHeader } from '@/components/ui/detail-header'
import { StatGrid } from '@/components/ui/stat-grid'
import { DetailSection } from '@/components/ui/detail-section'
import { ConditionsTable } from '@/components/ui/conditions-table'
import { YamlBlock } from '@/components/ui/yaml-block'

function healthColor(h: ReturnType<typeof targetRollupHealth>) {
  return h === 'healthy'
    ? ('success' as const)
    : h === 'degraded'
      ? ('warning' as const)
      : ('muted' as const)
}

function phaseColor(p: ReturnType<typeof renderTaskPhase>) {
  return p === 'succeeded'
    ? ('success' as const)
    : p === 'failed'
      ? ('danger' as const)
      : p === 'rendering'
        ? ('warning' as const)
        : ('muted' as const)
}

function hasData(v: unknown) {
  return v != null && (typeof v !== 'object' || Object.keys(v as object).length > 0)
}

export function TargetDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const navigate = useNavigate()
  const [showEdit, setShowEdit] = useState(false)
  const [showDelete, setShowDelete] = useState(false)

  const targetQ = useQuery(targetQueries.detail(namespace, name))
  const bindingsQ = useQuery(releaseBindingQueries.list(namespace))
  const renderTasksQ = useQuery(renderTaskQueries.list(namespace))
  const registryBindingsQ = useQuery(registryBindingQueries.list(namespace))

  const target = targetQ.data
  const health = targetRollupHealth(target?.status?.conditions)

  const boundBindings = useMemo(
    () =>
      (bindingsQ.data?.items ?? []).filter(
        (b) =>
          b.spec.targetRef.name === name &&
          (b.spec.targetRef.namespace ?? b.metadata.namespace) === namespace
      ),
    [bindingsQ.data, name, namespace]
  )

  const boundRegistryBindings = useMemo(
    () =>
      (registryBindingsQ.data?.items ?? []).filter(
        (b) =>
          b.spec.targetRef.name === name &&
          (b.spec.targetRef.namespace ?? b.metadata.namespace) === namespace
      ),
    [registryBindingsQ.data, name, namespace]
  )

  const rtByRelease = useMemo(() => {
    const m = new Map<string, RenderTask>()
    for (const rt of renderTasksQ.data?.items ?? []) {
      if (rt.spec.ownerName !== name) continue
      const last = (rt.spec.repository ?? '').split('/').pop() ?? ''
      if (last.startsWith('release-')) m.set(last.slice('release-'.length), rt)
    }
    return m
  }, [renderTasksQ.data, name])

  if (targetQ.isLoading) return <LoadingState icon={Server} label="Loading…" />

  if (targetQ.isError) {
    return <p className="text-destructive">Failed to load target.</p>
  }

  if (!target) {
    return <p className="text-destructive">Target not found.</p>
  }

  return (
    <div className="space-y-6">
      <DetailHeader
        icon={Server}
        title={name}
        namespace={namespace}
        subtitle={target.spec.renderRegistryRef.name}
        badges={
          <>
            <StatusDot color={healthColor(health)} />
            <Badge
              variant={
                health === 'healthy' ? 'success' : health === 'degraded' ? 'warning' : 'secondary'
              }
            >
              {health === 'healthy' ? 'Healthy' : health === 'degraded' ? 'Degraded' : 'Unknown'}
            </Badge>
          </>
        }
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
        backLabel="Back to Targets"
        onBack={() => navigate({ to: '/targets' })}
      />

      <StatGrid
        stats={[
          { label: 'Render Registry', value: target.spec.renderRegistryRef.name },
          { label: 'Bootstrap Version', value: target.status?.bootstrapVersion ?? '—' },
          {
            label: 'Created',
            value: new Date(target.metadata.creationTimestamp).toLocaleDateString(),
          },
          {
            label: 'Bound Releases',
            value: bindingsQ.isLoading
              ? '…'
              : bindingsQ.isError
                ? '–'
                : String(boundBindings.length),
          },
        ]}
      />

      <DetailSection
        title="Bound Releases"
        count={!bindingsQ.isLoading && !bindingsQ.isError ? boundBindings.length : undefined}
      >
        {bindingsQ.isLoading ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : bindingsQ.isError ? (
          <p className="text-sm text-destructive">Failed to load bindings.</p>
        ) : boundBindings.length === 0 ? (
          <p className="text-sm text-muted-foreground">No releases bound to this target.</p>
        ) : (
          <div className="space-y-2">
            {boundBindings.map((binding) => {
              const relName = binding.spec.releaseRef.name
              const rt = rtByRelease.get(relName)
              const hasPhase = !!rt?.status?.conditions?.length
              const phase = renderTaskPhase(rt?.status?.conditions)
              return (
                <div
                  key={binding.metadata.name}
                  className="flex items-center justify-between rounded-lg border border-border bg-card px-4 py-3"
                >
                  <div className="flex items-center gap-3">
                    <Package className="h-4 w-4 text-muted-foreground" />
                    <Link
                      to="/releases/$namespace/$name"
                      params={{ namespace, name: relName }}
                      className="text-sm font-medium text-foreground hover:text-primary transition-colors"
                    >
                      {relName}
                    </Link>
                    <Link
                      to="/releasebindings/$namespace/$name"
                      params={{ namespace, name: binding.metadata.name }}
                      className="text-xs text-muted-foreground hover:text-primary transition-colors"
                    >
                      View binding
                    </Link>
                  </div>
                  <div className="flex items-center gap-1.5">
                    {renderTasksQ.isLoading ? (
                      <span className="text-xs text-muted-foreground">…</span>
                    ) : renderTasksQ.isError ? (
                      <span className="text-xs text-destructive">Failed to load phase</span>
                    ) : hasPhase ? (
                      <>
                        <StatusDot color={phaseColor(phase)} />
                        <span
                          className={cn(
                            'text-xs capitalize text-muted-foreground',
                            phase === 'failed' && 'text-destructive'
                          )}
                        >
                          {phase}
                        </span>
                      </>
                    ) : null}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </DetailSection>

      {hasData(target.spec.userdata) && (
        <DetailSection title="User Data">
          <YamlBlock value={target.spec.userdata} />
        </DetailSection>
      )}

      <DetailSection
        title="Registry Bindings"
        count={
          !registryBindingsQ.isLoading && !registryBindingsQ.isError
            ? boundRegistryBindings.length
            : undefined
        }
      >
        {registryBindingsQ.isLoading ? (
          <p className="text-sm text-muted-foreground">Loading…</p>
        ) : registryBindingsQ.isError ? (
          <p className="text-sm text-destructive">Failed to load registry bindings.</p>
        ) : boundRegistryBindings.length === 0 ? (
          <p className="text-sm text-muted-foreground">No registry bindings.</p>
        ) : (
          <div className="space-y-2">
            {boundRegistryBindings.map((b) => (
              <div
                key={b.metadata.name}
                className="rounded-lg border border-border bg-card px-4 py-3 text-sm font-mono text-foreground"
              >
                {b.metadata.name}
              </div>
            ))}
          </div>
        )}
      </DetailSection>

      <DetailSection title="Conditions">
        <ConditionsTable conditions={target.status?.conditions} />
      </DetailSection>

      {showEdit && <EditTargetDialog open onOpenChange={setShowEdit} target={target} />}
      <DeleteTargetDialog
        open={showDelete}
        onOpenChange={setShowDelete}
        namespace={namespace}
        name={name}
      />
    </div>
  )
}
