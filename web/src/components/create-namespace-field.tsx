// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query'
import { namespaceQueries } from '@/api/queries'
import { inputCls } from '@/components/ui/controls'

/**
 * Namespace a create dialog writes into. With a namespace selected in the
 * sidebar it is fixed; in the "All namespaces" view (`fixed === null`) the
 * user has to pick one, so nothing lands in `default` by accident.
 */
export function CreateNamespaceField({
  fixed,
  value,
  onChange,
  error,
}: {
  fixed: string | null
  value: string
  onChange: (ns: string) => void
  error?: string
}) {
  const { data } = useQuery({ ...namespaceQueries.list(), enabled: fixed === null })

  if (fixed !== null) {
    return (
      <div className="rounded-md bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
        Creating in namespace <span className="font-mono text-foreground">{fixed}</span>
      </div>
    )
  }

  const namespaces =
    data?.items?.map((n) => n.metadata.name).sort((a, b) => a.localeCompare(b)) ?? []

  return (
    <div>
      <label
        htmlFor="create-namespace"
        className="mb-1 block text-xs font-medium text-muted-foreground"
      >
        Namespace
      </label>
      <select
        id="create-namespace"
        className={inputCls}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      >
        <option value="">Select a namespace…</option>
        {namespaces.map((ns) => (
          <option key={ns} value={ns}>
            {ns}
          </option>
        ))}
      </select>
      {error && <p className="mt-1 text-xs text-destructive">{error}</p>}
    </div>
  )
}
