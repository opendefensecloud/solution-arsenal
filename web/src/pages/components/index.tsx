import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { componentVersionQueries } from '@/api/queries'
import { useSSE } from '@/hooks/useSSE'
import { useNamespace } from '@/hooks/useNamespace'
import { useListState } from '@/hooks/useListState'
import { isForbiddenError } from '@/api/client'
import { ForbiddenAllNs } from '@/components/forbidden-all-ns'
import { Badge } from '@/components/ui/badge'
import { LoadingState } from '@/components/ui/loading-state'
import { ErrorState } from '@/components/ui/error-state'
import { EmptyState } from '@/components/ui/empty-state'
import { ListToolbar } from '@/components/ui/list-toolbar'
import { FilterPanel } from '@/components/ui/filter-panel'
import { Pagination } from '@/components/ui/pagination'
import { cn } from '@/lib/utils'
import { Boxes, Package, Globe } from 'lucide-react'
import { groupComponents } from './group'

const SORT_OPTIONS = [
  { label: 'Name', value: 'name' },
  { label: 'Age', value: 'age' },
]

export function ComponentsPage() {
  const { namespace } = useNamespace()
  const navigate = useNavigate()
  useSSE(namespace)
  const { data, isLoading, isError, error } = useQuery(componentVersionQueries.list(namespace))

  const ls = useListState()
  const [showFilter, setShowFilter] = useState(false)
  const [namespaceFilter, setNamespaceFilter] = useState<Set<string>>(new Set())

  const allComponents = useMemo(() => groupComponents(data?.items ?? []), [data])

  const allNamespaces = useMemo(
    () => Array.from(new Set(allComponents.map((c) => c.namespace))).sort(),
    [allComponents]
  )

  const effectiveNamespaceFilter = useMemo(() => {
    if (namespace !== null || namespaceFilter.size === 0) return namespaceFilter
    const pruned = new Set([...namespaceFilter].filter((ns) => allNamespaces.includes(ns)))
    return pruned.size === namespaceFilter.size ? namespaceFilter : pruned
  }, [namespace, namespaceFilter, allNamespaces])

  const filtered = useMemo(() => {
    let result = allComponents
    if (ls.search) {
      const q = ls.search.toLowerCase()
      result = result.filter(
        (c) =>
          c.name.toLowerCase().includes(q) ||
          c.repositories.some((r) => r.toLowerCase().includes(q)) ||
          c.registries.some((r) => r.toLowerCase().includes(q))
      )
    }
    if (effectiveNamespaceFilter.size > 0) {
      result = result.filter((c) => effectiveNamespaceFilter.has(c.namespace))
    }
    return [...result].sort((a, b) => {
      const cmp =
        ls.sortField === 'age'
          ? a.creationTimestamp.localeCompare(b.creationTimestamp)
          : a.name.localeCompare(b.name)
      return ls.sortDir === 'asc' ? cmp : -cmp
    })
  }, [allComponents, ls.search, ls.sortField, ls.sortDir, effectiveNamespaceFilter])

  const totalPages = ls.perPage === Infinity ? 1 : Math.ceil(filtered.length / ls.perPage)
  const paged =
    ls.perPage === Infinity
      ? filtered
      : filtered.slice((ls.page - 1) * ls.perPage, ls.page * ls.perPage)

  const activeFilterCount = effectiveNamespaceFilter.size > 0 ? 1 : 0

  if (namespace === null && isForbiddenError(error)) {
    return <ForbiddenAllNs resource="components" />
  }

  if (isLoading) return <LoadingState icon={Boxes} label="Loading components..." />
  if (isError) return <ErrorState message="Failed to load components. Please retry." />

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Components</h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            namespace <span className="font-mono">{namespace ?? 'all'}</span>
          </p>
        </div>
        <span className="rounded-md bg-secondary px-2.5 py-1 text-sm font-medium text-secondary-foreground">
          {allComponents.length} component
          {allComponents.length !== 1 ? 's' : ''}
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

          {allComponents.length === 0 ? (
            <EmptyState icon={Boxes} message="No components discovered yet" />
          ) : filtered.length === 0 ? (
            <EmptyState message="No components match your search." />
          ) : (
            <div
              className={cn(ls.tileView ? 'grid sm:grid-cols-2 lg:grid-cols-3 gap-3' : 'space-y-2')}
            >
              {paged.map((comp) => {
                const versionCount = comp.versionCount
                const key = `${comp.namespace}/${comp.name}`
                const handleClick = () =>
                  navigate({
                    to: '/components/$namespace/$name',
                    params: {
                      namespace: comp.namespace,
                      name: comp.name,
                    },
                  })
                if (ls.tileView) {
                  return (
                    <button
                      type="button"
                      key={key}
                      onClick={handleClick}
                      className="w-full text-left rounded-lg border border-border bg-card p-4 cursor-pointer hover:bg-accent/50 transition-colors"
                    >
                      <div className="flex items-center gap-3">
                        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                          <Package className="h-5 w-5 text-primary" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <h3 className="text-sm font-semibold text-foreground truncate">
                            {comp.name}
                          </h3>
                          <p className="text-xs text-muted-foreground font-mono truncate">
                            {comp.repositories.join(', ')}
                          </p>
                        </div>
                      </div>
                      <div className="mt-3 flex items-center gap-3 text-xs text-muted-foreground">
                        <Badge variant="secondary" className="text-[11px]">
                          {versionCount} {versionCount === 1 ? 'version' : 'versions'}
                        </Badge>
                        <span className="inline-flex items-center gap-1 truncate">
                          <Globe className="h-3 w-3 shrink-0" />
                          <span className="truncate">{comp.registries.join(', ')}</span>
                        </span>
                      </div>
                    </button>
                  )
                }
                return (
                  <div
                    key={key}
                    role="button"
                    tabIndex={0}
                    onClick={handleClick}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        handleClick()
                      }
                    }}
                    className="flex items-center gap-4 rounded-lg border border-border bg-card px-4 py-3 cursor-pointer hover:bg-accent/50 transition-colors"
                  >
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
                      <Package className="h-5 w-5 text-primary" />
                    </div>
                    <div className="min-w-0 flex-1">
                      <h3 className="text-sm font-semibold text-foreground">{comp.name}</h3>
                      <p className="text-xs text-muted-foreground font-mono truncate">
                        {comp.repositories.join(', ')}
                      </p>
                    </div>
                    <Badge variant="secondary" className="text-[11px] shrink-0">
                      {versionCount} {versionCount === 1 ? 'version' : 'versions'}
                    </Badge>
                    <span className="inline-flex items-center gap-1 text-xs text-muted-foreground shrink-0">
                      <Globe className="h-3 w-3" />
                      {comp.registries.join(', ')}
                    </span>
                  </div>
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
              <div className="max-h-40 space-y-0.5 overflow-auto">
                {allNamespaces.map((ns) => (
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
                ))}
              </div>
            </div>
          )}
        </FilterPanel>
      </div>
    </div>
  )
}
