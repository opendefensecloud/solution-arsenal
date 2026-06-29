// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo } from 'react'
import { useParams, useNavigate, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { registryQueries, targetQueries } from '@/api/queries'
import { StatusDot } from '@/components/ui/status-dot'
import { Badge } from '@/components/ui/badge'
import { ArrowLeft, Globe, Server, Lock, Unlock } from 'lucide-react'
import type { Condition } from '@/api/types'

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

export function RegistryDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const navigate = useNavigate()

  const registryQ = useQuery(registryQueries.detail(namespace, name))
  const targetsQ = useQuery(targetQueries.list(namespace))

  const registry = registryQ.data

  const boundTargets = useMemo(
    () =>
      (targetsQ.data?.items ?? []).filter(
        (t) => t.spec.renderRegistryRef.name === name && t.metadata.namespace === namespace
      ),
    [targetsQ.data, name, namespace]
  )

  if (registryQ.isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Globe className="h-4 w-4 animate-pulse" />
        Loading…
      </div>
    )
  }

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
      <button
        onClick={() => navigate({ to: '/registries' })}
        className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to Registries
      </button>

      <div className="flex items-start gap-4">
        <div className="rounded-xl bg-emerald-50 p-3 dark:bg-emerald-500/10">
          <Globe className="h-6 w-6 text-emerald-600 dark:text-emerald-400" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h1 className="text-2xl font-bold text-foreground">{name}</h1>
            <Badge variant="secondary">{namespace}</Badge>
            <div className="flex items-center gap-1.5">
              <StatusDot color={registry.spec.plainHTTP ? 'warning' : 'success'} />
              <Badge variant={registry.spec.plainHTTP ? 'warning' : 'success'}>
                {registry.spec.plainHTTP ? 'HTTP' : 'HTTPS'}
              </Badge>
            </div>
          </div>
          <p className="mt-1 text-sm text-muted-foreground font-mono">{url}</p>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        {[
          { label: 'Hostname', value: registry.spec.hostname },
          { label: 'Namespace', value: namespace },
          { label: 'Flavor', value: registry.spec.flavor ?? 'unknown' },
          { label: 'Targets', value: String(boundTargets.length) },
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
        ].map(({ label, value }) => (
          <div key={label} className="rounded-lg border border-border bg-card p-3">
            <p className="text-xs font-medium text-muted-foreground">{label}</p>
            <p className="mt-0.5 text-sm font-semibold text-foreground font-mono">{value}</p>
          </div>
        ))}
      </div>

      {/* Credentials */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Credentials
        </h2>
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
      </div>

      {/* Bound targets */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Targets using this Registry ({boundTargets.length})
        </h2>
        {boundTargets.length === 0 ? (
          <p className="text-sm text-muted-foreground">No targets use this registry.</p>
        ) : (
          <div className="space-y-2">
            {boundTargets.map((t) => (
              <div
                key={`${t.metadata.namespace}/${t.metadata.name}`}
                className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3"
              >
                <Server className="h-4 w-4 text-muted-foreground" />
                <Link
                  to="/targets/$namespace/$name"
                  params={{ namespace: t.metadata.namespace, name: t.metadata.name }}
                  className="text-sm font-medium text-foreground hover:text-primary transition-colors"
                >
                  {t.metadata.name}
                </Link>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Conditions */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Conditions
        </h2>
        <ConditionsTable conditions={registry.status?.conditions} />
      </div>
    </div>
  )
}
