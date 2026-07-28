import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { registryQueries, registryBindingQueries } from '@/api/queries'
import { useSSE } from '@/hooks/useSSE'
import { useNamespace } from '@/hooks/useNamespace'
import { useListState } from '@/hooks/useListState'
import { isForbiddenError } from '@/api/client'
import { ForbiddenAllNs } from '@/components/forbidden-all-ns'
import { StatusDot } from '@/components/ui/status-dot'
import { LoadingState } from '@/components/ui/loading-state'
import { ErrorState } from '@/components/ui/error-state'
import { EmptyState } from '@/components/ui/empty-state'
import { ListToolbar } from '@/components/ui/list-toolbar'
import { FilterPanel } from '@/components/ui/filter-panel'
import { Pagination } from '@/components/ui/pagination'
import { cn } from '@/lib/utils'
import { Globe } from 'lucide-react'

const SORT_OPTIONS = [
  { label: 'Name', value: 'name' },
  { label: 'Hostname', value: 'hostname' },
  { label: 'Last Synced', value: 'lastSynced' },
]

const CONNECTION_OPTIONS = [
  { value: 'secure', label: 'HTTPS' },
  { value: 'insecure', label: 'HTTP' },
]

export function RegistriesPage() {
  const { namespace } = useNamespace()
  const navigate = useNavigate()
  useSSE(namespace)
  const { data, isLoading, error } = useQuery(registryQueries.list(namespace))
  const { data: bindingsData, isError: isBindingsError } = useQuery(
    registryBindingQueries.list(namespace)
  )

  const ls = useListState()
  const [showFilter, setShowFilter] = useState(false)
  const [httpsFilter, setHttpsFilter] = useState<Set<string>>(new Set())
  const [flavorFilter, setFlavorFilter] = useState<Set<string>>(new Set())

  const allRegistries = useMemo(() => data?.items ?? [], [data])

  const allFlavors = useMemo(
    () =>
      Array.from(
        new Set(allRegistries.map((r) => r.spec.flavor).filter(Boolean) as string[])
      ).sort(),
    [allRegistries]
  )

  const bindingCounts = useMemo(() => {
    const counts = new Map<string, number>()
    for (const b of bindingsData?.items ?? []) {
      const refKey = `${b.metadata.namespace}/${b.spec.registryRef.name}`
      counts.set(refKey, (counts.get(refKey) ?? 0) + 1)
    }
    return counts
  }, [bindingsData])

  const filtered = useMemo(() => {
    let result = allRegistries
    if (ls.search) {
      const q = ls.search.toLowerCase()
      result = result.filter(
        (r) =>
          r.metadata.name.toLowerCase().includes(q) || r.spec.hostname.toLowerCase().includes(q)
      )
    }
    if (httpsFilter.size > 0) {
      const wantSecure = httpsFilter.has('secure')
      const wantInsecure = httpsFilter.has('insecure')
      result = result.filter(
        (r) => (wantSecure && !r.spec.plainHTTP) || (wantInsecure && r.spec.plainHTTP)
      )
    }
    if (flavorFilter.size > 0) {
      result = result.filter((r) => flavorFilter.has(r.spec.flavor ?? ''))
    }
    return [...result].sort((a, b) => {
      let cmp: number
      if (ls.sortField === 'hostname') {
        cmp = a.spec.hostname.localeCompare(b.spec.hostname)
      } else if (ls.sortField === 'lastSynced') {
        const aSync = a.status?.lastSynced ?? ''
        const bSync = b.status?.lastSynced ?? ''
        cmp = aSync.localeCompare(bSync)
      } else {
        cmp = a.metadata.name.localeCompare(b.metadata.name)
      }
      return ls.sortDir === 'asc' ? cmp : -cmp
    })
  }, [allRegistries, ls.search, ls.sortField, ls.sortDir, httpsFilter, flavorFilter])

  const totalPages = ls.perPage === Infinity ? 1 : Math.ceil(filtered.length / ls.perPage)
  const safeTotalPages = Math.max(totalPages, 1)
  const safePage = Math.min(ls.page, safeTotalPages)
  const paged =
    ls.perPage === Infinity
      ? filtered
      : filtered.slice((safePage - 1) * ls.perPage, safePage * ls.perPage)

  const lsPage = ls.page
  const lsSetPage = ls.setPage
  useEffect(() => {
    if (lsPage > safeTotalPages) lsSetPage(safeTotalPages)
  }, [lsPage, safeTotalPages, lsSetPage])

  const activeFilterCount = (httpsFilter.size > 0 ? 1 : 0) + (flavorFilter.size > 0 ? 1 : 0)

  if (namespace === null && isForbiddenError(error)) {
    return <ForbiddenAllNs resource="registries" />
  }

  if (error) return <ErrorState message="Failed to load registries. Please retry." />
  if (isLoading) return <LoadingState icon={Globe} label="Loading registries..." />

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Registries</h1>
          <p className="mt-0.5 text-xs text-muted-foreground">
            namespace <span className="font-mono">{namespace ?? 'all'}</span>
          </p>
        </div>
        <span className="rounded-md bg-secondary px-2.5 py-1 text-sm font-medium text-secondary-foreground">
          {allRegistries.length} registr
          {allRegistries.length !== 1 ? 'ies' : 'y'}
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

          {allRegistries.length === 0 ? (
            <EmptyState icon={Globe} message="No registries found" />
          ) : filtered.length === 0 ? (
            <EmptyState message="No registries match your search." />
          ) : (
            <div
              className={cn(ls.tileView ? 'grid sm:grid-cols-2 lg:grid-cols-3 gap-3' : 'space-y-2')}
            >
              {paged.map((reg) => {
                const flavor = reg.spec.flavor ?? 'unknown'
                const scheme = reg.spec.plainHTTP ? 'http' : 'https'
                const url = `${scheme}://${reg.spec.hostname}`
                const dotColor = reg.spec.plainHTTP ? ('warning' as const) : ('success' as const)
                const dotLabel = reg.spec.plainHTTP ? 'HTTP' : 'HTTPS'
                const key = `${reg.metadata.namespace}/${reg.metadata.name}`
                const bindings =
                  bindingCounts.get(`${reg.metadata.namespace}/${reg.metadata.name}`) ?? 0
                const bindingText = isBindingsError
                  ? 'targets unavailable'
                  : `${bindings} binding${bindings !== 1 ? 's' : ''}`
                return (
                  <button
                    type="button"
                    key={key}
                    onClick={() =>
                      navigate({
                        to: '/registries/$namespace/$name',
                        params: { namespace: reg.metadata.namespace, name: reg.metadata.name },
                      })
                    }
                    className={cn(
                      'w-full cursor-pointer rounded-lg border border-border bg-card p-4 text-left transition-all hover:shadow-md hover:border-primary/30',
                      ls.tileView && 'h-full'
                    )}
                  >
                    {ls.tileView ? (
                      <div className="flex flex-col h-full">
                        <div className="flex items-center gap-3">
                          <Globe className="h-5 w-5 shrink-0 text-muted-foreground" />
                          <h3 className="text-sm font-semibold text-foreground truncate flex-1">
                            {reg.metadata.name}
                          </h3>
                        </div>
                        <p className="mt-1.5 text-xs text-muted-foreground font-mono truncate">
                          {url}
                        </p>
                        <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground flex-1">
                          <StatusDot color={dotColor} label={dotLabel} />
                          <span>{flavor}</span>
                          <span>{bindingText}</span>
                        </div>
                      </div>
                    ) : (
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3 min-w-0 flex-1">
                          <Globe className="h-5 w-5 shrink-0 text-muted-foreground" />
                          <div className="min-w-0 flex-1">
                            <h3 className="text-base font-semibold text-foreground">
                              {reg.metadata.name}
                            </h3>
                            <p className="mt-0.5 text-sm text-muted-foreground font-mono truncate">
                              {url}
                            </p>
                          </div>
                        </div>
                        <div className="flex items-center gap-3 shrink-0 ml-4">
                          <div className="flex flex-col items-end gap-1">
                            <StatusDot color={dotColor} label={dotLabel} />
                            <p className="text-[11px] text-muted-foreground">
                              {flavor} &middot; {bindingText}
                            </p>
                          </div>
                        </div>
                      </div>
                    )}
                  </button>
                )
              })}
            </div>
          )}

          <Pagination
            page={safePage}
            totalPages={safeTotalPages}
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
          <div>
            <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Connection
            </p>
            <div className="space-y-1">
              {CONNECTION_OPTIONS.map(({ value, label }) => (
                <label
                  key={value}
                  className="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-xs text-foreground hover:bg-accent transition-colors"
                >
                  <input
                    type="checkbox"
                    checked={httpsFilter.has(value)}
                    onChange={() => {
                      const next = new Set(httpsFilter)
                      if (next.has(value)) next.delete(value)
                      else next.add(value)
                      setHttpsFilter(next)
                      ls.setPage(1)
                    }}
                    className="h-3.5 w-3.5 rounded border-border accent-primary"
                  />
                  {label}
                </label>
              ))}
            </div>
          </div>
          {allFlavors.length > 0 && (
            <div>
              <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Flavor
              </p>
              <div className="space-y-1">
                {allFlavors.map((f) => (
                  <label
                    key={f}
                    className="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-xs text-foreground hover:bg-accent transition-colors capitalize"
                  >
                    <input
                      type="checkbox"
                      checked={flavorFilter.has(f)}
                      onChange={() => {
                        const next = new Set(flavorFilter)
                        if (next.has(f)) next.delete(f)
                        else next.add(f)
                        setFlavorFilter(next)
                        ls.setPage(1)
                      }}
                      className="h-3.5 w-3.5 rounded border-border accent-primary"
                    />
                    {f}
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
