// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo } from 'react'
import { useParams, useNavigate, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { profileQueries, releaseBindingQueries, targetQueries, releaseQueries } from '@/api/queries'
import { StatusDot } from '@/components/ui/status-dot'
import { Badge } from '@/components/ui/badge'
import { targetRollupHealth } from '@/lib/utils'
import { Users, Server, Package } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
import { BackButton } from '@/components/ui/back-button'
import type { Target } from '@/api/types'

function healthColor(h: ReturnType<typeof targetRollupHealth>) {
  return h === 'healthy'
    ? ('success' as const)
    : h === 'degraded'
      ? ('warning' as const)
      : ('muted' as const)
}

export function ProfileDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const navigate = useNavigate()

  const profileQ = useQuery(profileQueries.detail(namespace, name))
  const bindingsQ = useQuery(releaseBindingQueries.list(namespace))
  const targetsQ = useQuery(targetQueries.list(null))
  const releasesQ = useQuery(releaseQueries.list(namespace))

  const profile = profileQ.data

  // ReleaseBindings where releaseRef matches profile's releaseRef
  const matchedBindings = useMemo(() => {
    if (!profile) return []
    const releaseRef = profile.spec.releaseRef.name
    return (bindingsQ.data?.items ?? []).filter((b) => b.spec.releaseRef.name === releaseRef)
  }, [profile, bindingsQ.data])

  // Unique targets from those bindings
  const matchedTargetRefs = useMemo(() => {
    const seen = new Set<string>()
    return matchedBindings
      .map((b) => ({
        name: b.spec.targetRef.name,
        namespace: b.spec.targetNamespace ?? b.metadata.namespace,
      }))
      .filter(({ name: n, namespace: ns }) => {
        const key = `${ns}/${n}`
        if (seen.has(key)) return false
        seen.add(key)
        return true
      })
  }, [matchedBindings])

  const targetMap = useMemo(() => {
    const m = new Map<string, Target>()
    for (const t of targetsQ.data?.items ?? [])
      m.set(`${t.metadata.namespace}/${t.metadata.name}`, t)
    return m
  }, [targetsQ.data])

  const releaseExists = useMemo(
    () =>
      releasesQ.data?.items.some((r) => r.metadata.name === profile?.spec.releaseRef.name) ?? false,
    [releasesQ.data, profile]
  )

  if (profileQ.isLoading) return <LoadingState icon={Users} label="Loading…" />

  if (profileQ.isError) {
    return <p className="text-destructive">Failed to load profile.</p>
  }

  if (!profile) {
    return <p className="text-destructive">Profile not found.</p>
  }

  const matchLabels = profile.spec.targetSelector?.matchLabels ?? {}

  return (
    <div className="space-y-6">
      <BackButton label="Back to Profiles" onClick={() => navigate({ to: '/profiles' })} />

      <div className="flex items-start gap-4">
        <div className="rounded-xl bg-amber-50 p-3 dark:bg-amber-500/10">
          <Users className="h-6 w-6 text-amber-600 dark:text-amber-400" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <h1 className="text-2xl font-bold text-foreground">{name}</h1>
            <Badge variant="secondary">{namespace}</Badge>
            <Badge variant="secondary">
              {profile.status?.matchedTargets ?? matchedTargetRefs.length} matched
            </Badge>
          </div>
          <p className="mt-1 text-sm text-muted-foreground">
            Deploys release{' '}
            <span className="font-mono font-medium text-foreground">
              {profile.spec.releaseRef.name}
            </span>
          </p>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        {[
          { label: 'Release Ref', value: profile.spec.releaseRef.name },
          { label: 'Namespace', value: namespace },
          {
            label: 'Matched Targets',
            value: String(profile.status?.matchedTargets ?? matchedTargetRefs.length),
          },
          {
            label: 'Created',
            value: new Date(profile.metadata.creationTimestamp).toLocaleDateString(),
          },
        ].map(({ label, value }) => (
          <div key={label} className="rounded-lg border border-border bg-card p-3">
            <p className="text-xs font-medium text-muted-foreground">{label}</p>
            <p className="mt-0.5 text-sm font-semibold text-foreground font-mono">{value}</p>
          </div>
        ))}
      </div>

      {/* Target selector */}
      {Object.keys(matchLabels).length > 0 && (
        <div>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
            Target Selector
          </h2>
          <div className="flex flex-wrap gap-2">
            {Object.entries(matchLabels).map(([k, v]) => (
              <Badge key={k} variant="secondary" className="font-mono">
                {k}={v}
              </Badge>
            ))}
          </div>
        </div>
      )}

      {/* Release link */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Release
        </h2>
        <div className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3">
          <Package className="h-4 w-4 text-muted-foreground" />
          {releaseExists ? (
            <Link
              to="/releases/$namespace/$name"
              params={{ namespace, name: profile.spec.releaseRef.name }}
              className="text-sm font-medium text-foreground hover:text-primary transition-colors"
            >
              {profile.spec.releaseRef.name}
            </Link>
          ) : (
            <span className="text-sm font-medium text-foreground font-mono">
              {profile.spec.releaseRef.name}
            </span>
          )}
        </div>
      </div>

      {/* Matched targets */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
          Matched Targets ({matchedTargetRefs.length})
        </h2>
        {matchedTargetRefs.length === 0 ? (
          <p className="text-sm text-muted-foreground">No targets matched.</p>
        ) : (
          <div className="space-y-2">
            {matchedTargetRefs.map(({ name: tName, namespace: tNs }) => {
              const target = targetMap.get(`${tNs}/${tName}`)
              const health = targetRollupHealth(target?.status?.conditions)
              return (
                <div
                  key={`${tNs}/${tName}`}
                  className="flex items-center justify-between rounded-lg border border-border bg-card px-4 py-3"
                >
                  <div className="flex items-center gap-3">
                    <Server className="h-4 w-4 text-muted-foreground" />
                    <Link
                      to="/targets/$namespace/$name"
                      params={{ namespace: tNs, name: tName }}
                      className="text-sm font-medium text-foreground hover:text-primary transition-colors"
                    >
                      {tName}
                    </Link>
                    {tNs !== namespace && (
                      <Badge variant="secondary" className="text-xs">
                        {tNs}
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center gap-1.5">
                    <StatusDot color={healthColor(health)} />
                    <span className="text-xs capitalize text-muted-foreground">{health}</span>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
