// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo, useState } from 'react'
import { useParams, useNavigate, Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { profileQueries, releaseBindingQueries, targetQueries, releaseQueries } from '@/api/queries'
import { Badge } from '@/components/ui/badge'
import { targetRollupHealth } from '@/lib/utils'
import { Users, Package } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
import { DetailHeader } from '@/components/ui/detail-header'
import { StatGrid } from '@/components/ui/stat-grid'
import { DetailSection } from '@/components/ui/detail-section'
import { ConditionsTable } from '@/components/ui/conditions-table'
import { YamlBlock } from '@/components/ui/yaml-block'
import type { Target } from '@/api/types'
import { EditProfileDialog } from './edit-profile-dialog'
import { DeleteProfileDialog } from './delete-profile-dialog'

function healthVariant(h: ReturnType<typeof targetRollupHealth>) {
  return h === 'healthy'
    ? ('success' as const)
    : h === 'degraded'
      ? ('warning' as const)
      : ('secondary' as const)
}

function hasData(v: unknown) {
  return v != null && (typeof v !== 'object' || Object.keys(v as object).length > 0)
}

export function ProfileDetailPage() {
  const { namespace, name } = useParams({ strict: false }) as { namespace: string; name: string }
  const navigate = useNavigate()
  const [showEdit, setShowEdit] = useState(false)
  const [showDelete, setShowDelete] = useState(false)

  const profileQ = useQuery(profileQueries.detail(namespace, name))
  const bindingsQ = useQuery(releaseBindingQueries.list(namespace))
  const targetsQ = useQuery(targetQueries.list(null))
  const releasesQ = useQuery(releaseQueries.list(namespace))

  const profile = profileQ.data

  // ReleaseBindings owned by this profile
  const matchedBindings = useMemo(() => {
    if (!profile) return []
    return (bindingsQ.data?.items ?? []).filter((b) =>
      b.metadata.ownerReferences?.some(
        (o) => o.kind === 'Profile' && o.name === profile.metadata.name
      )
    )
  }, [profile, bindingsQ.data])

  // Unique targets from those bindings
  const matchedTargetRefs = useMemo(() => {
    const seen = new Set<string>()
    return matchedBindings
      .map((b) => ({
        name: b.spec.targetRef.name,
        namespace: b.spec.targetRef.namespace ?? b.metadata.namespace,
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

  if (bindingsQ.isError || targetsQ.isError || releasesQ.isError) {
    return <p className="text-destructive">Failed to load related pipeline resources.</p>
  }

  if (!profile) {
    return <p className="text-destructive">Profile not found.</p>
  }

  const matchLabels = profile.spec.targetSelector?.matchLabels ?? {}

  return (
    <div className="space-y-6">
      <DetailHeader
        icon={Users}
        title={name}
        namespace={namespace}
        subtitle={`${profile.spec.releaseRef.name} · ${namespace}`}
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
        backLabel="Back to Profiles"
        onBack={() => navigate({ to: '/profiles' })}
      />

      <StatGrid
        stats={[
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
        ]}
      />

      {Object.keys(matchLabels).length > 0 && (
        <DetailSection title="Target Selector">
          <div className="rounded-lg border border-border bg-card p-4">
            <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">
              Match Labels
            </p>
            <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
              {Object.entries(matchLabels).map(([k, v]) => (
                <div key={k} className="contents">
                  <span className="font-mono text-foreground">{k}</span>
                  <span className="font-mono text-muted-foreground">= {v}</span>
                </div>
              ))}
            </div>
          </div>
        </DetailSection>
      )}

      <DetailSection title="Release">
        <div className="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-3">
          <Package className="h-4 w-4 text-muted-foreground" />
          {releaseExists ? (
            <Link
              to="/releases/$namespace/$name"
              params={{ namespace, name: profile.spec.releaseRef.name }}
              className="text-sm font-medium text-foreground hover:text-primary transition-colors font-mono"
            >
              {profile.spec.releaseRef.name}
            </Link>
          ) : (
            <span className="text-sm font-medium text-foreground font-mono">
              {profile.spec.releaseRef.name}
            </span>
          )}
        </div>
      </DetailSection>

      <DetailSection title="Matched Targets" count={matchedTargetRefs.length}>
        {matchedTargetRefs.length === 0 ? (
          <div className="rounded-lg border-2 border-dashed border-border p-8 text-center">
            <p className="text-sm text-muted-foreground">No targets matched.</p>
          </div>
        ) : (
          <div className="grid gap-2">
            {matchedTargetRefs.map(({ name: tName, namespace: tNs }) => {
              const target = targetMap.get(`${tNs}/${tName}`)
              const health = targetRollupHealth(target?.status?.conditions)
              return (
                <div
                  key={`${tNs}/${tName}`}
                  className="rounded-lg border border-border bg-card p-4"
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <Link
                        to="/targets/$namespace/$name"
                        params={{ namespace: tNs, name: tName }}
                        className="text-sm font-semibold text-foreground hover:text-primary transition-colors"
                      >
                        {tName}
                      </Link>
                      <p className="text-xs text-muted-foreground">{tNs}</p>
                    </div>
                    <Badge variant={healthVariant(health)}>{health}</Badge>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </DetailSection>

      {hasData(profile.spec.userdata) && (
        <DetailSection title="User Data">
          <YamlBlock value={profile.spec.userdata} />
        </DetailSection>
      )}

      <DetailSection title="Conditions">
        <ConditionsTable conditions={profile.status?.conditions} />
      </DetailSection>

      {showEdit && <EditProfileDialog open onOpenChange={setShowEdit} profile={profile} />}
      <DeleteProfileDialog
        open={showDelete}
        onOpenChange={setShowDelete}
        namespace={namespace}
        name={name}
      />
    </div>
  )
}
