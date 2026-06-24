import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import type { ResourceEvent } from '@/api/types'
import { useAuth } from '@/hooks/useAuth'

/**
 * Opens an SSE connection to the BFF. A `null` namespace opens the
 * cluster-wide stream (`/api/events`); a string opens the namespace-scoped
 * stream. Events include the originating namespace either way, but we
 * invalidate by resource name only (prefix match) so both the namespaced
 * cache key and the cluster-wide "*" key get refreshed.
 */
export function useSSE(namespace: string | null) {
  const queryClient = useQueryClient()
  // Re-key on the impersonated user so the EventSource is torn down and
  // reopened when admins switch "Preview as". The BFF starts its watches
  // with the session's identity at connection time, so an existing stream
  // would otherwise keep delivering events as the previous user.
  const { impersonatedUsername, impersonatedGroups } = useAuth()
  // Groups can change without the username changing (admin switches the
  // preview-as groups), so fold both into the effect key to force a reconnect.
  const impersonationKey = `${impersonatedUsername ?? ''}|${(impersonatedGroups ?? [])
    .slice()
    .sort()
    .join(',')}`

  useEffect(() => {
    const url = namespace === null ? '/api/events' : `/api/namespaces/${namespace}/events`
    const source = new EventSource(url)

    source.onmessage = (event) => {
      try {
        const data: ResourceEvent = JSON.parse(event.data)
        // Prefix-invalidate: any active query for this resource type
        // (namespaced or cluster-wide) gets marked stale and refetched.
        queryClient.invalidateQueries({ queryKey: [data.resource] })
      } catch {
        // ignore parse errors
      }
    }

    source.onerror = () => {
      // EventSource auto-reconnects
    }

    return () => source.close()
  }, [namespace, queryClient, impersonationKey])
}
