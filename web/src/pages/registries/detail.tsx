// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo } from 'react'
import { useParams, useNavigate, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { registryQueries, registryBindingQueries } from '@/api/queries'
import { StatusDot } from '@/components/ui/status-dot'
import { Badge } from '@/components/ui/badge'
import { Globe, Server, Lock, Unlock } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
import { DetailHeader } from '@/components/ui/detail-header'
import { StatGrid } from '@/components/ui/stat-grid'
import { DetailSection } from '@/components/ui/detail-section'
import { ConditionsTable } from '@/components/ui/conditions-table'

export function RegistryDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const navigate = useNavigate()

  const registryQ = useQuery(registryQueries.detail(namespace, name))
  const registryBindingsQ = useQuery(registryBindingQueries.list(namespace))

  const registry = registryQ.data

  const boundBindings = useMemo(
    () =>
      (registryBindingsQ.data?.items ?? []).filter(
        (b) => b.spec.registryRef.name === name && b.metadata.namespace === namespace
      ),
    [registryBindingsQ.data, name, namespace]
  )

  if (registryQ.isLoading) return <LoadingState icon={Globe} label="Loading…" />

  if (registryQ.isError) {
    return <p className="text-destructive">Failed to load registry.</p>
  }

  if (!registry) {
    return <p className="text-destructive">Registry not found.</p>
  }

  const scheme = registry.spec.plainHTTP ? 'http' : 'https'
  const url = `${scheme}://${registry.spec.hostname}`
  const hasCredentials = !!registry.spec.solarSecretRef

  return (
    <div className="space-y-6">
      <DetailHeader
        icon={Globe}
        title={name}
        namespace={namespace}
        subtitle={url}
        badges={
          <div className="flex items-center gap-1.5">
            <StatusDot color={registry.spec.plainHTTP ? 'warning' : 'success'} />
            <Badge variant={registry.spec.plainHTTP ? 'warning' : 'success'}>
              {registry.spec.plainHTTP ? 'HTTP' : 'HTTPS'}
            </Badge>
          </div>
        }
        backLabel="Back to Registries"
        onBack={() => navigate({ to: '/registries' })}
      />

      <StatGrid
        stats={[
          { label: 'Hostname', value: registry.spec.hostname },
          { label: 'Namespace', value: namespace },
          { label: 'Flavor', value: registry.spec.flavor ?? 'unknown' },
          {
            label: 'Targets',
            value: registryBindingsQ.isLoading
              ? '…'
              : registryBindingsQ.isError
                ? '–'
                : String(boundBindings.length),
          },
          {
            label: 'Created',
            value: new Date(registry.metadata.creationTimestamp).toLocaleDateString(),
          },
          ...(registry.status?.lastSynced
            ? [
                {
                  label: 'Last Synced',
                  value: new Date(registry.status.lastSynced).toLocaleString(),
                },
              ]
            : []),
        ]}
      />

      <DetailSection title="Credentials">
        <div className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3">
          {hasCredentials ? (
            <>
              <Lock className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
              <span className="text-sm text-foreground">
                Secret:{' '}
                <span className="font-mono font-medium">{registry.spec.solarSecretRef!.name}</span>
              </span>
            </>
          ) : (
            <>
              <Unlock className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm text-muted-foreground">No credentials configured</span>
            </>
          )}
        </div>
      </DetailSection>

      <DetailSection
        title="Targets using this Registry"
        count={
          !registryBindingsQ.isLoading && !registryBindingsQ.isError
            ? boundBindings.length
            : undefined
        }
      >
        {registryBindingsQ.isLoading ? (
          <p className="text-sm text-muted-foreground">Loading targets…</p>
        ) : registryBindingsQ.isError ? (
          <p className="text-sm text-destructive">Failed to load targets.</p>
        ) : boundBindings.length === 0 ? (
          <p className="text-sm text-muted-foreground">No targets use this registry.</p>
        ) : (
          <div className="space-y-2">
            {boundBindings.map((b) => {
              const tName = b.spec.targetRef.name
              const tNs = b.spec.targetRef.namespace ?? b.metadata.namespace
              return (
                <div
                  key={`${tNs}/${tName}`}
                  className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3"
                >
                  <Server className="h-4 w-4 text-muted-foreground" />
                  <div className="min-w-0">
                    <Link
                      to="/targets/$namespace/$name"
                      params={{ namespace: tNs, name: tName }}
                      className="text-sm font-medium text-foreground hover:text-primary transition-colors"
                    >
                      {tName}
                    </Link>
                    {tNs !== namespace && (
                      <p className="text-xs text-muted-foreground font-mono">{tNs}</p>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </DetailSection>

      <DetailSection title="Conditions">
        <ConditionsTable conditions={registry.status?.conditions} />
      </DetailSection>
    </div>
  )
}
