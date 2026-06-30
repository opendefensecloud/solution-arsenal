// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Navigate, useNavigate, useParams } from '@tanstack/react-router'
import { componentQueries, componentVersionQueries } from '@/api/queries'
import { useSSE } from '@/hooks/useSSE'
import { isForbiddenError } from '@/api/client'
import { Badge } from '@/components/ui/badge'
import { ArrowLeft, Globe, Package, Search } from 'lucide-react'
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
  const { data: versionsData, isError: versionsError } = useQuery(
    componentVersionQueries.list(namespace)
  )

  const [search, setSearch] = useState('')

  const versions = useMemo(() => {
    const list = (versionsData?.items ?? []).filter((v) => v.spec.componentRef.name === name)
    return list.sort((a, b) => b.spec.tag.localeCompare(a.spec.tag, undefined, { numeric: true }))
  }, [versionsData, name])

  const filtered = useMemo(() => {
    if (!search) return versions
    const q = search.toLowerCase()
    return versions.filter(
      (cv) =>
        cv.spec.tag.toLowerCase().includes(q) || primaryRepository(cv).toLowerCase().includes(q)
    )
  }, [versions, search])

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-muted-foreground">
        <Package className="h-4 w-4 animate-pulse" />
        Loading...
      </div>
    )
  }

  if (isError && isForbiddenError(error)) {
    return <Navigate to="/components" />
  }

  if (isError || versionsError) {
    return (
      <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4">
        <p className="text-sm text-destructive">Failed to load component. Please retry.</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <button
          onClick={() => navigate({ to: '/components' })}
          className="flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <div className="flex items-center gap-3">
          <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
            <Package className="h-6 w-6 text-primary" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-2xl font-bold text-foreground">{name}</h1>
              <Badge variant="secondary">
                {versions.length} {versions.length === 1 ? 'version' : 'versions'}
              </Badge>
            </div>
            <p className="text-sm text-muted-foreground font-mono">{comp?.spec.repository}</p>
          </div>
        </div>
      </div>

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
        <div className="rounded-lg border-2 border-dashed border-border p-8 text-center">
          <p className="text-sm text-muted-foreground">
            {versions.length === 0
              ? 'No versions discovered yet.'
              : 'No versions match your search.'}
          </p>
        </div>
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
    </div>
  )
}
