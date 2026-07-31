// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useParams, useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { renderTaskQueries } from '@/api/queries'
import { DetailHeader } from '@/components/ui/detail-header'
import { StatGrid } from '@/components/ui/stat-grid'
import { DetailSection } from '@/components/ui/detail-section'
import { ConditionsTable } from '@/components/ui/conditions-table'
import { Badge } from '@/components/ui/badge'
import { LoadingState } from '@/components/ui/loading-state'
import { renderTaskPhase } from '@/lib/utils'
import { Loader } from 'lucide-react'

function phaseVariant(p: ReturnType<typeof renderTaskPhase>) {
  return p === 'succeeded'
    ? ('success' as const)
    : p === 'failed'
      ? ('destructive' as const)
      : p === 'rendering'
        ? ('warning' as const)
        : ('secondary' as const)
}

export function RenderTaskDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const navigate = useNavigate()
  const rtQ = useQuery(renderTaskQueries.detail(namespace, name))
  const rt = rtQ.data

  if (rtQ.isLoading) return <LoadingState icon={Loader} label="Loading…" />
  if (rtQ.isError) return <p className="text-destructive">Failed to load render task.</p>
  if (!rt) return <p className="text-destructive">Render task not found.</p>

  const phase = renderTaskPhase(rt.status?.conditions)
  const job = rt.status?.jobRef

  return (
    <div className="space-y-6">
      <DetailHeader
        icon={Loader}
        title={name}
        namespace={namespace}
        subtitle={
          rt.spec.ownerName ? `${rt.spec.ownerKind ?? 'Owner'}: ${rt.spec.ownerName}` : undefined
        }
        badges={<Badge variant={phaseVariant(phase)}>{phase}</Badge>}
        backLabel="Back to Pipeline"
        onBack={() => navigate({ to: '/pipeline' })}
      />

      <StatGrid
        stats={[
          { label: 'Owner', value: rt.spec.ownerName ?? '—' },
          { label: 'Repository', value: rt.spec.repository ?? '—' },
          { label: 'Tag', value: rt.spec.tag ?? '—' },
          { label: 'Job', value: job?.name ?? '—' },
        ]}
      />

      <DetailSection title="Spec">
        <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 rounded-lg border border-border bg-card p-4 text-sm font-mono">
          <span className="text-muted-foreground">baseURL</span>
          <span className="text-foreground">{rt.spec.baseURL}</span>
          {rt.spec.ownerNamespace && (
            <>
              <span className="text-muted-foreground">ownerNamespace</span>
              <span className="text-foreground">{rt.spec.ownerNamespace}</span>
            </>
          )}
          {rt.status?.chartURL && (
            <>
              <span className="text-muted-foreground">chartURL</span>
              <span className="text-foreground break-all">{rt.status.chartURL}</span>
            </>
          )}
        </div>
      </DetailSection>

      <DetailSection title="Conditions">
        <ConditionsTable conditions={rt.status?.conditions} />
      </DetailSection>
    </div>
  )
}
