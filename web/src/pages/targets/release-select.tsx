// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query'
import { releaseQueries } from '@/api/queries'

export function ReleaseSelect({
  namespace,
  value,
  onChange,
}: {
  namespace: string
  value: Set<string>
  onChange: (v: Set<string>) => void
}) {
  const releasesQ = useQuery(releaseQueries.list(namespace))
  const releases = releasesQ.data?.items ?? []

  const toggle = (name: string) => {
    const next = new Set(value)
    if (next.has(name)) next.delete(name)
    else next.add(name)
    onChange(next)
  }

  if (releases.length === 0) {
    return (
      <p className="rounded-md border border-dashed border-border px-3 py-2 text-xs text-muted-foreground">
        No releases in this namespace.
      </p>
    )
  }

  return (
    <div className="max-h-40 space-y-0.5 overflow-auto rounded-md border border-border p-1">
      {releases.map((r) => (
        <label
          key={r.metadata.name}
          className="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-sm text-foreground transition-colors hover:bg-accent"
        >
          <input
            type="checkbox"
            checked={value.has(r.metadata.name)}
            onChange={() => toggle(r.metadata.name)}
            className="h-3.5 w-3.5 rounded border-border accent-primary"
          />
          <span className="font-mono">{r.metadata.name}</span>
        </label>
      ))}
    </div>
  )
}
