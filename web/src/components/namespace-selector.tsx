import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { namespaceQueries } from '@/api/queries'
import { useNamespace } from '@/hooks/useNamespace'
import { useAuth } from '@/hooks/useAuth'
import { cn } from '@/lib/utils'

const ALL_VALUE = '__all__'

export function NamespaceSelector() {
  const { namespace, setNamespace } = useNamespace()
  const { canListAllNamespaces, isAuthenticated } = useAuth()
  // Don't fire /api/namespaces until the user is authenticated — otherwise
  // the 401 would trigger client.ts's auto-login redirect, which races with
  // any manual login flow already in progress (e.g. an OIDC e2e test).
  const { data, isLoading, isError } = useQuery({
    ...namespaceQueries.list(),
    enabled: isAuthenticated,
  })

  const namespaces =
    data?.items?.map((n) => n.metadata.name).sort((a, b) => a.localeCompare(b)) ?? []

  // Reconcile the persisted selection with what the current identity is
  // actually allowed to see:
  //   - "All" is only valid if the user can list namespaces cluster-wide;
  //     otherwise pick the first accessible namespace.
  //   - A specific namespace must exist in the filtered list; otherwise
  //     fall back to "All" (admin) or first accessible (persona).
  useEffect(() => {
    if (!data) return

    if (namespace === null && !canListAllNamespaces) {
      // "All" no longer permitted — pick something else.
      if (namespaces.length > 0) setNamespace(namespaces[0])
      return
    }

    if (namespace !== null && !namespaces.includes(namespace)) {
      // Stale persisted ns; recover.
      if (canListAllNamespaces) {
        setNamespace(null)
      } else if (namespaces.length > 0) {
        setNamespace(namespaces[0])
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data, canListAllNamespaces])

  return (
    <div className="px-3 pb-2 pt-3">
      <label
        htmlFor="ns-selector"
        className="mb-1 block text-[11px] font-semibold uppercase tracking-wider text-muted-foreground"
      >
        Namespace
      </label>
      <select
        id="ns-selector"
        disabled={isLoading || isError}
        value={namespace ?? ALL_VALUE}
        onChange={(e) => setNamespace(e.target.value === ALL_VALUE ? null : e.target.value)}
        className={cn(
          'w-full rounded-md border border-sidebar-border bg-background px-2 py-1.5 text-sm text-foreground',
          'focus:outline-none focus:ring-2 focus:ring-ring',
          'disabled:cursor-not-allowed disabled:opacity-50'
        )}
      >
        {canListAllNamespaces && <option value={ALL_VALUE}>All namespaces</option>}
        {namespaces.map((ns) => (
          <option key={ns} value={ns}>
            {ns}
          </option>
        ))}
      </select>
      {isError && (
        <p className="mt-1 text-[11px] leading-tight text-destructive">Failed to list namespaces</p>
      )}
    </div>
  )
}
