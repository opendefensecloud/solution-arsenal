import { createContext, useContext } from 'react'

/**
 * Value semantics:
 *   - `null`    → "All namespaces" (BFF calls the cluster-wide list endpoints)
 *   - `string`  → a specific Kubernetes namespace name
 */
export type NamespaceValue = string | null

export interface NamespaceContextValue {
  namespace: NamespaceValue
  setNamespace: (ns: NamespaceValue) => void
}

export const NamespaceContext = createContext<NamespaceContextValue | undefined>(undefined)

export function useNamespace(): NamespaceContextValue {
  const ctx = useContext(NamespaceContext)
  if (!ctx) {
    throw new Error('useNamespace must be used inside <NamespaceProvider>')
  }
  return ctx
}
