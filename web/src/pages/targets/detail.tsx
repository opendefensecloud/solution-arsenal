// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo } from 'react'
import { useParams, useNavigate, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { targetQueries, releaseBindingQueries, renderTaskQueries } from '@/api/queries'
import { StatusDot } from '@/components/ui/status-dot'
import { Badge } from '@/components/ui/badge'
import { cn, targetRollupHealth, renderTaskPhase } from '@/lib/utils'
import { Server, Package } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
import { BackButton } from '@/components/ui/back-button'
import type { Condition, RenderTask } from '@/api/types'

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

function ConditionsTable({ conditions }: { conditions?: Condition[] }) {
  if (!conditions?.length) return <p className="text-sm text-muted-foreground">No conditions</p>
  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border bg-muted/30">
            <th className="px-3 py-2 text-left font-medium text-muted-foreground">Type</th>
            <th className="px-3 py-2 text-left font-medium text-muted-foreground">Status</th>
            <th className="px-3 py-2 text-left font-medium text-muted-foreground">Reason</th>
            <th className="px-3 py-2 text-left font-medium text-muted-foreground">Message</th>
          </tr>
        </thead>
        <tbody>
          {conditions.map((c) => (
            <tr key={c.type} className="border-b border-border last:border-b-0">
              <td className="px-3 py-2 font-mono font-medium text-foreground">{c.type}</td>
              <td className="px-3 py-2">
                <StatusDot
                  color={
                    c.status === 'True' ? 'success' : c.status === 'False' ? 'danger' : 'muted'
                  }
                  label={c.status}
                />
              </td>
              <td className="px-3 py-2 font-mono text-muted-foreground">{c.reason}</td>
              <td className="px-3 py-2 text-muted-foreground">{c.message || '—'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function TargetDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const navigate = useNavigate()

  const targetQ = useQuery(targetQueries.detail(namespace, name))
  const bindingsQ = useQuery(releaseBindingQueries.list(namespace))
  const renderTasksQ = useQuery(renderTaskQueries.list(namespace))

  const target = targetQ.data
  const health = targetRollupHealth(target?.status?.conditions)

  const boundBindings = useMemo(
    () =>
      (bindingsQ.data?.items ?? []).filter(
        (b) =>
          b.spec.targetRef.name === name &&
          (b.spec.targetNamespace ?? b.metadata.namespace) === namespace
      ),
    [bindingsQ.data, name, namespace]
  )

  // Release name encoded in spec.repository as last segment: "{targetNs}/{relNs}/release-{relName}"
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
      <BackButton label="Back to Targets" onClick={() => navigate({ to: '/targets' })} />

      <div className="flex items-start gap-4">
        <div className="rounded-xl bg-blue-50 p-3 dark:bg-blue-500/10">
          <Server className="h-6 w-6 text-blue-600 dark:text-blue-400" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h1 className="text-2xl font-bold text-foreground">{name}</h1>
            <Badge variant="secondary">{namespace}</Badge>
            <div className="flex items-center gap-1.5">
              <StatusDot color={healthColor(health)} />
              <Badge
                variant={
                  health === 'healthy' ? 'success' : health === 'degraded' ? 'warning' : 'secondary'
                }
              >
                {health === 'healthy' ? 'Healthy' : health === 'degraded' ? 'Degraded' : 'Unknown'}
              </Badge>
            </div>
          </div>
          <p className="mt-1 text-sm text-muted-foreground font-mono">
            Registry: {target.spec.renderRegistryRef.name}
          </p>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        {[
          { label: 'Render Registry', value: target.spec.renderRegistryRef.name },
          { label: 'Namespace', value: namespace },
          {
            label: 'Created',
            value: new Date(target.metadata.creationTimestamp).toLocaleDateString(),
          },
          {
            label: 'Bound Releases',
            value: bindingsQ.isLoading ? '…' : bindingsQ.isError ? '–' : String(boundBindings.length),
          },
        ].map(({ label, value }) => (
          <div key={label} className="rounded-lg border border-border bg-card p-3">
            <p className="text-xs font-medium text-muted-foreground">{label}</p>
            <p className="mt-0.5 text-sm font-semibold text-foreground font-mono">{value}</p>
          </div>
        ))}
      </div>

      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Bound Releases{!bindingsQ.isLoading && !bindingsQ.isError && ` (${boundBindings.length})`}
        </h2>
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
                  </div>
                  <div className="flex items-center gap-1.5">
                    {renderTasksQ.isLoading ? (
                      <span className="text-xs text-muted-foreground">…</span>
                    ) : renderTasksQ.isError ? (
                      <span className="text-xs text-destructive">Failed to load phase</span>
                    ) : (
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
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Conditions
        </h2>
        <ConditionsTable conditions={target.status?.conditions} />
      </div>
    </div>
  )
}
