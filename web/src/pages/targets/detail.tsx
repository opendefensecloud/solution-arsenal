// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo, useState } from 'react'
import { useParams, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { targetQueries, registryBindingQueries } from '@/api/queries'
import { HealthBadge } from '@/components/ui/health-badge'
import { Server } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
import { DeleteTargetDialog } from './delete-target-dialog'
import { EditTargetDialog } from './edit-target-dialog'
import { AgentStat } from './agent-stat'
import { ReleasesReadyStat } from './releases-ready-stat'
import { BoundReleaseList } from './bound-releases'
import { useBoundReleases } from './use-bound-releases'
import { DetailHeader } from '@/components/ui/detail-header'
import { StatGrid } from '@/components/ui/stat-grid'
import { DetailSection } from '@/components/ui/detail-section'
import { ConditionsTable } from '@/components/ui/conditions-table'
import { YamlBlock } from '@/components/ui/yaml-block'

function hasData(v: unknown) {
  return v != null && (typeof v !== 'object' || Object.keys(v as object).length > 0)
}

export function TargetDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const navigate = useNavigate()
  const [showEdit, setShowEdit] = useState(false)
  const [showDelete, setShowDelete] = useState(false)

  const targetQ = useQuery(targetQueries.detail(namespace, name))
  const registryBindingsQ = useQuery(registryBindingQueries.list(namespace))

  const target = targetQ.data
  const bound = useBoundReleases(namespace, name, target)

  const boundRegistryBindings = useMemo(
    () =>
      (registryBindingsQ.data?.items ?? []).filter(
        (b) =>
          b.spec.targetRef.name === name &&
          (b.spec.targetRef.namespace ?? b.metadata.namespace) === namespace
      ),
    [registryBindingsQ.data, name, namespace]
  )

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
        badges={<HealthBadge conditions={target.status?.conditions} />}
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
            value: bound.isLoading ? '…' : bound.isError ? '–' : String(bound.rows.length),
          },
          {
            label: 'Agent',
            value: (
              <AgentStat namespace={namespace} name={name} conditions={target.status?.conditions} />
            ),
          },
          {
            label: 'Releases ready',
            value: <ReleasesReadyStat conditions={target.status?.conditions} />,
          },
        ]}
      />

      <DetailSection
        title="Bound Releases"
        count={!bound.isLoading && !bound.isError ? bound.rows.length : undefined}
      >
        <BoundReleaseList bound={bound} />
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
