// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useParams, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { releaseBindingQueries } from '@/api/queries'
import { DetailHeader } from '@/components/ui/detail-header'
import { DetailSection } from '@/components/ui/detail-section'
import { ConditionsTable } from '@/components/ui/conditions-table'
import { LoadingState } from '@/components/ui/loading-state'
import { Link2, Server, Package } from 'lucide-react'

export function ReleaseBindingDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const bindingQ = useQuery(releaseBindingQueries.detail(namespace, name))
  const binding = bindingQ.data

  if (bindingQ.isLoading) return <LoadingState icon={Link2} label="Loading…" />
  if (bindingQ.isError) return <p className="text-destructive">Failed to load release binding.</p>
  if (!binding) return <p className="text-destructive">Release binding not found.</p>

  const targetNs = binding.spec.targetRef.namespace ?? namespace
  const targetName = binding.spec.targetRef.name
  const releaseName = binding.spec.releaseRef.name

  return (
    <div className="space-y-6">
      <DetailHeader
        icon={Link2}
        title={name}
        namespace={namespace}
        subtitle={`${targetName} → ${releaseName}`}
        backLabel="Back"
        onBack={() => window.history.back()}
      />

      <DetailSection title="Target">
        <div className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3">
          <Server className="h-4 w-4 text-muted-foreground" />
          <Link
            to="/targets/$namespace/$name"
            params={{ namespace: targetNs, name: targetName }}
            className="text-sm font-medium text-foreground hover:text-primary transition-colors font-mono"
          >
            {targetName}
          </Link>
        </div>
      </DetailSection>

      <DetailSection title="Release">
        <div className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3">
          <Package className="h-4 w-4 text-muted-foreground" />
          <Link
            to="/releases/$namespace/$name"
            params={{ namespace, name: releaseName }}
            className="text-sm font-medium text-foreground hover:text-primary transition-colors font-mono"
          >
            {releaseName}
          </Link>
        </div>
      </DetailSection>

      <DetailSection title="Conditions">
        <ConditionsTable conditions={binding.status?.conditions} />
      </DetailSection>
    </div>
  )
}
