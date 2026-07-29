// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Navigate, Link, useNavigate, useParams } from '@tanstack/react-router'
import { componentQueries, componentVersionQueries, releaseQueries } from '@/api/queries'
import { useSSE } from '@/hooks/useSSE'
import { isForbiddenError } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { Globe, Package, Search } from 'lucide-react'
import { LoadingState } from '@/components/ui/loading-state'
import { ErrorState } from '@/components/ui/error-state'
import { EmptyState } from '@/components/ui/empty-state'
import { DetailHeader } from '@/components/ui/detail-header'
import { DetailSection } from '@/components/ui/detail-section'
import type { ComponentVersion } from '@/api/types'

function primaryRepository(cv: ComponentVersion): string {
  const resources = cv.spec.resources
  if (!resources) return ''
  const key = cv.spec.entrypoint?.resourceName ?? Object.keys(resources)[0]
  return resources[key]?.repository ?? ''
}

export function ComponentVersionsPage() {
  const { namespace, name } = useParams({ strict: false }) as {
    namespace: string
    name: string
  }
  const navigate = useNavigate()
  useSSE(namespace)

  const {
    data: comp,
    isLoading,
    isError,
    error,
  } = useQuery(componentQueries.detail(namespace, name))
  const {
    data: versionsData,
    isLoading: versionsLoading,
    isError: versionsError,
  } = useQuery(componentVersionQueries.list(namespace))

  const releasesQ = useQuery(releaseQueries.list(namespace))

  const [search, setSearch] = useState('')

  const versions = useMemo(() => {
    const list = (versionsData?.items ?? []).filter((v) => v.spec.componentRef.name === name)
    return list.sort((a, b) => b.spec.tag.localeCompare(a.spec.tag, undefined, { numeric: true }))
  }, [versionsData, name])

  const versionNames = useMemo(() => new Set(versions.map((v) => v.metadata.name)), [versions])
  const relatedReleases = useMemo(
    () =>
      (releasesQ.data?.items ?? []).filter((r) =>
        versionNames.has(r.spec.componentVersionRef.name)
      ),
    [releasesQ.data, versionNames]
  )

  const filtered = useMemo(() => {
    if (!search) return versions
    const q = search.toLowerCase()
    return versions.filter(
      (cv) =>
        cv.spec.tag.toLowerCase().includes(q) || primaryRepository(cv).toLowerCase().includes(q)
    )
  }, [versions, search])

  if (isLoading || versionsLoading) return <LoadingState icon={Package} label="Loading..." />
  if (isError && isForbiddenError(error)) return <Navigate to="/components" />
  if (isError || versionsError)
    return <ErrorState message="Failed to load component. Please retry." />

  return (
    <div className="space-y-6">
      <DetailHeader
        icon={Package}
        title={name}
        namespace={namespace}
        subtitle={comp?.spec.repository}
        badges={
          <Badge variant="secondary">
            {versions.length} {versions.length === 1 ? 'version' : 'versions'}
          </Badge>
        }
        backLabel="Back to Components"
        onBack={() => navigate({ to: '/components' })}
      />

      <div className="relative max-w-md">
        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <input
          type="text"
          placeholder="Search versions..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-full rounded-lg border border-input bg-background py-2 pl-10 pr-4 text-sm text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring transition-colors"
        />
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          message={
            versions.length === 0 ? 'No versions discovered yet.' : 'No versions match your search.'
          }
        />
      ) : (
        <div className="rounded-lg border border-border divide-y divide-border">
          {filtered.map((cv) => (
            <div key={cv.metadata.name} className="flex items-center gap-3 px-4 py-3">
              <div className="flex shrink-0 items-center justify-center rounded-md bg-muted px-2 py-1">
                <span className="text-xs font-mono font-semibold text-foreground">
                  {cv.spec.tag}
                </span>
              </div>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-mono text-foreground truncate">
                  {primaryRepository(cv)}
                </p>
              </div>
              {comp?.spec.registry && (
                <span className="inline-flex items-center gap-1 text-xs text-muted-foreground shrink-0">
                  <Globe className="h-3 w-3" />
                  {comp.spec.registry}
                </span>
              )}
            </div>
          ))}
        </div>
      )}

      <DetailSection title="Related Releases" count={relatedReleases.length}>
        {relatedReleases.length === 0 ? (
          <p className="text-sm text-muted-foreground">No releases reference these versions.</p>
        ) : (
          <div className="space-y-2">
            {relatedReleases.map((r) => (
              <div
                key={`${r.metadata.namespace}/${r.metadata.name}`}
                className="rounded-lg border border-border bg-card px-4 py-3"
              >
                <Link
                  to="/releases/$namespace/$name"
                  params={{ namespace: r.metadata.namespace, name: r.metadata.name }}
                  className="text-sm font-medium text-foreground hover:text-primary transition-colors font-mono"
                >
                  {r.metadata.name}
                </Link>
              </div>
            ))}
          </div>
        )}
      </DetailSection>
    </div>
  )
}
