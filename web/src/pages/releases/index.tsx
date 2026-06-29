import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { releaseQueries } from '@/api/queries'
import { useSSE } from '@/hooks/useSSE'
import { useNamespace } from '@/hooks/useNamespace'
import { useListState } from '@/hooks/useListState'
import { isForbiddenError } from '@/api/client'
import { ForbiddenAllNs } from '@/components/forbidden-all-ns'
import { Badge } from '@/components/ui/badge'
import { StatusBadge } from '@/components/ui/status-badge'
import { LoadingState } from '@/components/ui/loading-state'
import { ErrorState } from '@/components/ui/error-state'
import { EmptyState } from '@/components/ui/empty-state'
import { ListToolbar } from '@/components/ui/list-toolbar'
import { FilterPanel } from '@/components/ui/filter-panel'
import { Pagination } from '@/components/ui/pagination'
import { cn } from '@/lib/utils'
import { Package } from 'lucide-react'

const SORT_OPTIONS = [
  { label: 'Name', value: 'name' },
  { label: 'Age', value: 'age' },
]

export function ReleasesPage() {
  const { namespace } = useNamespace()
  const navigate = useNavigate()
  useSSE(namespace)
  const { data, isLoading, error } = useQuery(releaseQueries.list(namespace))

  const ls = useListState()
  const [showFilter, setShowFilter] = useState(false)
  const [namespaceFilter, setNamespaceFilter] = useState<Set<string>>(new Set())
  const [nsSearch, setNsSearch] = useState('')

  const allReleases = useMemo(() => data?.items ?? [], [data])

  const allNamespaces = useMemo(
    () => Array.from(new Set(allReleases.map((r) => r.metadata.namespace))).sort(),
    [allReleases]
  )

  const visibleNamespaces = useMemo(
    () =>
      nsSearch
        ? allNamespaces.filter((ns) => ns.toLowerCase().includes(nsSearch.toLowerCase()))
        : allNamespaces,
    [allNamespaces, nsSearch]
  )

  const effectiveNamespaceFilter = useMemo(() => {
    if (namespace !== null || namespaceFilter.size === 0) return namespaceFilter
    const pruned = new Set([...namespaceFilter].filter((ns) => allNamespaces.includes(ns)))
    return pruned.size === namespaceFilter.size ? namespaceFilter : pruned
  }, [namespace, namespaceFilter, allNamespaces])

  const filtered = useMemo(() => {
    let result = allReleases
    if (ls.search) {
      const q = ls.search.toLowerCase()
      result = result.filter(
        (r) =>
          r.metadata.name.toLowerCase().includes(q) ||
          r.spec.componentVersionRef.name.toLowerCase().includes(q)
      )
    }
    if (effectiveNamespaceFilter.size > 0) {
      result = result.filter((r) => effectiveNamespaceFilter.has(r.metadata.namespace))
    }
    return [...result].sort((a, b) => {
      const cmp =
        ls.sortField === 'age'
          ? a.metadata.creationTimestamp.localeCompare(b.metadata.creationTimestamp)
          : a.metadata.name.localeCompare(b.metadata.name)
      return ls.sortDir === 'asc' ? cmp : -cmp
    })
  }, [allReleases, ls.search, ls.sortField, ls.sortDir, effectiveNamespaceFilter])

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
    if (filtered.length === 0) return
    if (lsPage > totalPages) lsSetPage(totalPages)
  }, [filtered.length, lsPage, lsPerPage, lsSetPage, totalPages])

  const activeFilterCount = effectiveNamespaceFilter.size > 0 ? 1 : 0

  if (namespace === null && isForbiddenError(error)) {
    return <ForbiddenAllNs resource="releases" />
  }

  if (error) return <ErrorState message="Failed to load releases. Please retry." />
  if (isLoading) return <LoadingState icon={Package} label="Loading releases..." />

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Releases</h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            namespace <span className="font-mono">{namespace ?? 'all'}</span>
          </p>
        </div>
        <span className="rounded-md bg-secondary px-2.5 py-1 text-sm font-medium text-secondary-foreground">
          {allReleases.length} release{allReleases.length !== 1 ? 's' : ''}
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

          {allReleases.length === 0 ? (
            <EmptyState icon={Package} message="No releases found" />
          ) : filtered.length === 0 ? (
            <EmptyState message="No releases match your search." />
          ) : (
            <div
              className={cn(ls.tileView ? 'grid sm:grid-cols-2 lg:grid-cols-3 gap-3' : 'space-y-2')}
            >
              {paged.map((release) => (
                <button
                  type="button"
                  key={`${release.metadata.namespace}/${release.metadata.name}`}
                  onClick={() => navigate({ to: '/releases/$namespace/$name', params: { namespace: release.metadata.namespace, name: release.metadata.name } })}
                  className={cn(
                    'w-full cursor-pointer rounded-lg border border-border bg-card p-4 text-left transition-all hover:shadow-md hover:border-primary/30',
                    ls.tileView && 'h-full'
                  )}
                >
                  {ls.tileView ? (
                    <div className="flex flex-col h-full">
                      <div className="flex items-start justify-between gap-2">
                        <h3 className="text-sm font-semibold text-foreground truncate">
                          {release.metadata.name}
                        </h3>
                        <Badge variant="secondary" className="shrink-0 text-[11px]">
                          {release.spec.componentVersionRef.name}
                        </Badge>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground flex-1">
                        {release.metadata.namespace}
                      </p>
                      <div className="mt-2 flex items-center justify-end">
                        <StatusBadge
                          conditions={release.status?.conditions}
                          type="ComponentVersionResolved"
                        />
                      </div>
                    </div>
                  ) : (
                    <div className="flex items-center justify-between">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-3">
                          <h3 className="text-base font-semibold text-foreground">
                            {release.metadata.name}
                          </h3>
                          <Badge variant="secondary" className="text-[11px]">
                            {release.spec.componentVersionRef.name}
                          </Badge>
                        </div>
                        <p className="mt-0.5 text-sm text-muted-foreground">
                          {release.metadata.namespace}
                        </p>
                      </div>
                      <StatusBadge
                        conditions={release.status?.conditions}
                        type="ComponentVersionResolved"
                      />
                    </div>
                  )}
                </button>
              ))}
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
          {allNamespaces.length > 1 && (
            <div>
              <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Namespace
              </p>
              <input
                type="text"
                placeholder="Search namespace..."
                value={nsSearch}
                onChange={(e) => setNsSearch(e.target.value)}
                className="mb-2 w-full rounded-md border border-input bg-background py-1.5 px-2.5 text-xs text-foreground placeholder:text-muted-foreground focus:border-ring focus:outline-none"
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
