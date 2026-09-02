// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react'

interface Stat {
  label: string
  value: ReactNode
}

export function StatGrid({ stats }: { stats: Stat[] }) {
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
      {stats.map(({ label, value }) => (
        <div key={label} className="rounded-lg border border-border bg-background px-4 py-3">
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            {label}
          </p>
          <p className="mt-1 text-lg font-semibold text-foreground">{value}</p>
        </div>
      ))}
    </div>
  )
}
