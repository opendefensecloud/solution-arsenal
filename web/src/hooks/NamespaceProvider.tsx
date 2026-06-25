import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { NamespaceContext, type NamespaceContextValue, type NamespaceValue } from './useNamespace'

const STORAGE_KEY = 'solar-ui-selected-namespace'

// Query-key roots that are scoped to the selected namespace. Switching
// namespaces resets only these, leaving cross-namespace queries like
// "auth/me" and "namespaces" untouched to avoid needless refetch churn.
const NAMESPACE_SCOPED_KEYS = [
  'targets',
  'releases',
  'releasebindings',
  'components',
  'componentversions',
  'registries',
  'profiles',
  'rendertasks',
]

function loadInitial(): NamespaceValue {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw === null) return null
    // Sentinel for "all"; everything else is a literal namespace.
    return raw === '*' ? null : raw
  } catch {
    return null
  }
}

export function NamespaceProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [namespace, setNamespaceState] = useState<NamespaceValue>(loadInitial)

  // Persist + reset resource queries on switch. Resetting (not just
  // invalidating) clears the cache so the UI shows loading instead of the
  // previous namespace's rows for a beat.
  const setNamespace = useCallback(
    (next: NamespaceValue) => {
      setNamespaceState(next)
      try {
        localStorage.setItem(STORAGE_KEY, next ?? '*')
      } catch {
        // localStorage can throw in private windows / quota issues; ignore.
      }
      queryClient.resetQueries({
        predicate: (query) => NAMESPACE_SCOPED_KEYS.includes(String(query.queryKey[0] ?? '')),
      })
    },
    [queryClient]
  )

  // Persist on mount so the sentinel "*" gets normalised even if the user
  // never touches the selector.
  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, namespace ?? '*')
    } catch {
      /* ignore */
    }
    // Intentionally empty deps: only runs once on first render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const value = useMemo<NamespaceContextValue>(
    () => ({ namespace, setNamespace }),
    [namespace, setNamespace]
  )

  return <NamespaceContext.Provider value={value}>{children}</NamespaceContext.Provider>
}
