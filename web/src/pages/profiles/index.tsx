import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { profileQueries } from '@/api/queries'
import { useSSE } from '@/hooks/useSSE'
import { useNamespace } from '@/hooks/useNamespace'
import { useListState } from '@/hooks/useListState'
import { isForbiddenError } from '@/api/client'
import { ForbiddenAllNs } from '@/components/forbidden-all-ns'
import { LoadingState } from '@/components/ui/loading-state'
import { EmptyState } from '@/components/ui/empty-state'
import { ListToolbar } from '@/components/ui/list-toolbar'
import { FilterPanel } from '@/components/ui/filter-panel'
import { Pagination } from '@/components/ui/pagination'
import { cn, formatDate } from '@/lib/utils'
import { Users } from 'lucide-react'

const SORT_OPTIONS = [
  { label: 'Name', value: 'name' },
  { label: 'Age', value: 'age' },
  { label: 'Targets', value: 'targets' },
]

export function ProfilesPage() {
  const { namespace } = useNamespace()
  const navigate = useNavigate()
  useSSE(namespace)
  const { data, isLoading, error } = useQuery(profileQueries.list(namespace))

  const ls = useListState()
  const [showFilter, setShowFilter] = useState(false)
  const [namespaceFilter, setNamespaceFilter] = useState<Set<string>>(new Set())
  const [nsSearch, setNsSearch] = useState('')

  const allProfiles = useMemo(() => data?.items ?? [], [data])

  const allNamespaces = useMemo(
    () => Array.from(new Set(allProfiles.map((p) => p.metadata.namespace))).sort(),
    [allProfiles]
  )

  const visibleNamespaces = useMemo(
    () =>
      nsSearch
        ? allNamespaces.filter((ns) => ns.toLowerCase().includes(nsSearch.toLowerCase()))
        : allNamespaces,
    [allNamespaces, nsSearch]
  )

  const filtered = useMemo(() => {
    let result = allProfiles
    if (ls.search) {
      const q = ls.search.toLowerCase()
      result = result.filter(
        (p) =>
          p.metadata.name.toLowerCase().includes(q) ||
          p.spec.releaseRef.name.toLowerCase().includes(q)
      )
    }
    if (namespace === null && allNamespaces.length > 1 && namespaceFilter.size > 0) {
      result = result.filter((p) => namespaceFilter.has(p.metadata.namespace))
    }
    return [...result].sort((a, b) => {
      const cmp =
        ls.sortField === 'age'
          ? a.metadata.creationTimestamp.localeCompare(b.metadata.creationTimestamp)
          : ls.sortField === 'targets'
            ? (a.status?.matchedTargets ?? 0) - (b.status?.matchedTargets ?? 0)
            : a.metadata.name.localeCompare(b.metadata.name)
      return ls.sortDir === 'asc' ? cmp : -cmp
    })
  }, [
    allProfiles,
    ls.search,
    ls.sortField,
    ls.sortDir,
    namespaceFilter,
    namespace,
    allNamespaces.length,
  ])

  const totalPages = ls.perPage === Infinity ? 1 : Math.ceil(filtered.length / ls.perPage)
  const paged =
    ls.perPage === Infinity
      ? filtered
      : filtered.slice((ls.page - 1) * ls.perPage, ls.page * ls.perPage)

  const lsPage = ls.page
  const lsPerPage = ls.perPage
  const lsSetPage = ls.setPage
  useEffect(() => {
    if (lsPerPage === Infinity) return
    const nextTotalPages = Math.max(1, Math.ceil(filtered.length / lsPerPage))
    if (lsPage > nextTotalPages) lsSetPage(nextTotalPages)
  }, [filtered.length, lsPage, lsPerPage, lsSetPage])

  const activeFilterCount =
    namespace === null && allNamespaces.length > 1 && namespaceFilter.size > 0 ? 1 : 0

  if (namespace === null && isForbiddenError(error)) {
    return <ForbiddenAllNs resource="profiles" />
  }

  if (isLoading) return <LoadingState icon={Users} label="Loading profiles..." />

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Profiles</h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            namespace <span className="font-mono">{namespace ?? 'all'}</span>
          </p>
        </div>
        <span className="rounded-md bg-secondary px-2.5 py-1 text-sm font-medium text-secondary-foreground">
          {allProfiles.length} profile{allProfiles.length !== 1 ? 's' : ''}
        </span>
      </div>

      <div className="flex gap-0">
        <div className="flex-1 min-w-0 space-y-4">
          <ListToolbar
            search={ls.search}
            onSearch={ls.setSearch}
            showFilter={showFilter}
            onToggleFilter={() => setShowFilter((v) => !v)}
            activeFilterCount={activeFilterCount}
            tileView={ls.tileView}
            onSetTileView={ls.setTileView}
          />

          {allProfiles.length === 0 ? (
            <EmptyState icon={Users} message="No profiles found" />
          ) : filtered.length === 0 ? (
            <EmptyState message="No profiles match your search." />
          ) : (
            <div
              className={cn(ls.tileView ? 'grid sm:grid-cols-2 lg:grid-cols-3 gap-3' : 'space-y-2')}
            >
              {paged.map((profile) => {
                const matchedTargets = profile.status?.matchedTargets ?? 0
                const key = `${profile.metadata.namespace}/${profile.metadata.name}`
                const matchLabels = Object.entries(profile.spec.targetSelector?.matchLabels ?? {})
                return (
                  <button
                    type="button"
                    key={key}
                    onClick={() =>
                      navigate({
                        to: '/profiles/$namespace/$name',
                        params: {
                          namespace: profile.metadata.namespace,
                          name: profile.metadata.name,
                        },
                      })
                    }
                    className={cn(
                      'w-full cursor-pointer rounded-lg border border-border bg-card p-4 text-left transition-all hover:shadow-md hover:border-primary/30',
                      ls.tileView && 'h-full'
                    )}
                  >
                    {ls.tileView ? (
                      <div className="flex flex-col h-full">
                        <h3 className="text-sm font-semibold text-foreground truncate">
                          {profile.metadata.name}
                        </h3>
                        <p className="mt-1.5 text-xs text-muted-foreground flex-1">
                          {profile.metadata.namespace}
                        </p>
                        <p className="text-xs text-muted-foreground font-mono truncate">
                          {profile.spec.releaseRef.name}
                        </p>
                        {matchLabels.length > 0 && (
                          <div className="mt-1.5 flex flex-wrap gap-1">
                            {matchLabels.map(([k, v]) => (
                              <span
                                key={k}
                                className="inline-flex items-center rounded bg-muted px-1.5 py-0.5 text-[10px] font-mono"
                              >
                                <span className="text-foreground">{k}</span>
                                <span className="text-muted-foreground">={v}</span>
                              </span>
                            ))}
                          </div>
                        )}
                        <div className="mt-2 flex items-center justify-between">
                          <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <span>
                              {matchedTargets} target{matchedTargets !== 1 ? 's' : ''}
                            </span>
                            <span>{formatDate(profile.metadata.creationTimestamp)}</span>
                          </div>
                          <span className="rounded-md bg-secondary px-1.5 py-0.5 text-[11px] font-medium text-secondary-foreground">
                            {matchedTargets} matched
                          </span>
                        </div>
                      </div>
                    ) : (
                      <div className="flex items-start justify-between gap-4">
                        <div className="min-w-0">
                          <h3 className="text-base font-semibold text-foreground">
                            {profile.metadata.name}
                          </h3>
                          <p className="text-sm text-muted-foreground">
                            {profile.metadata.namespace} &middot; {profile.spec.releaseRef.name}{' '}
                            &middot; {matchedTargets} target{matchedTargets !== 1 ? 's' : ''}{' '}
                            &middot; {formatDate(profile.metadata.creationTimestamp)}
                          </p>
                          {matchLabels.length > 0 && (
                            <div className="mt-1.5 flex flex-wrap gap-1">
                              {matchLabels.map(([k, v]) => (
                                <span
                                  key={k}
                                  className="inline-flex items-center rounded bg-muted px-1.5 py-0.5 text-[10px] font-mono"
                                >
                                  <span className="text-foreground">{k}</span>
                                  <span className="text-muted-foreground">={v}</span>
                                </span>
                              ))}
                            </div>
                          )}
                        </div>
                        <span className="shrink-0 rounded-md bg-secondary px-2 py-1 text-xs font-medium text-secondary-foreground">
                          {matchedTargets} matched
                        </span>
                      </div>
                    )}
                  </button>
                )
              })}
            </div>
          )}

          <Pagination
            page={ls.page}
            totalPages={totalPages}
            perPage={ls.perPage}
            filteredCount={filtered.length}
            perPageOptions={ls.perPageOptions}
            onPage={ls.setPage}
            onPerPage={ls.setPerPage}
          />
        </div>

        <FilterPanel open={showFilter} onClose={() => setShowFilter(false)} title="Filter / Sort">
          <div>
            <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Sort By
            </p>
            <div className="flex flex-wrap gap-2">
              {SORT_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  onClick={() => ls.toggleSort(opt.value)}
                  className={cn(
                    'flex items-center gap-1 rounded-md border px-2.5 py-1.5 text-xs font-medium transition-colors',
                    ls.sortField === opt.value
                      ? 'border-primary/40 bg-primary/5 text-primary'
                      : 'border-border text-muted-foreground hover:text-foreground'
                  )}
                >
                  {opt.label}
                  {ls.sortField === opt.value && <span>{ls.sortDir === 'asc' ? '↑' : '↓'}</span>}
                </button>
              ))}
            </div>
          </div>
          {namespace === null && allNamespaces.length > 1 && (
            <div>
              <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Namespace
              </p>
              <input
                type="text"
                placeholder="Search namespace..."
                value={nsSearch}
                onChange={(e) => setNsSearch(e.target.value)}
                className="mb-2 w-full rounded-md border border-input bg-background py-1.5 px-2.5 text-xs text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
              />
              <div className="max-h-40 space-y-0.5 overflow-auto">
                {visibleNamespaces.length === 0 ? (
                  <p className="px-2 py-3 text-center text-xs text-muted-foreground">No match</p>
                ) : (
                  visibleNamespaces.map((ns) => (
                    <label
                      key={ns}
                      className="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-xs text-foreground hover:bg-accent transition-colors"
                    >
                      <input
                        type="checkbox"
                        checked={namespaceFilter.has(ns)}
                        onChange={() => {
                          const next = new Set(namespaceFilter)
                          if (next.has(ns)) next.delete(ns)
                          else next.add(ns)
                          setNamespaceFilter(next)
                          ls.setPage(1)
                        }}
                        className="h-3.5 w-3.5 rounded border-border accent-primary"
                      />
                      {ns}
                    </label>
                  ))
                )}
              </div>
            </div>
          )}
        </FilterPanel>
      </div>
    </div>
  )
}
