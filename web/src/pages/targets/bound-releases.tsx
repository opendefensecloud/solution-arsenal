// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { BoundReleases } from './use-bound-releases'
import { ReleaseRow } from './release-row'

// BoundReleaseList renders the rows of useBoundReleases.
export function BoundReleaseList({ bound }: { bound: BoundReleases }) {
  if (bound.isLoading) return <p className="text-sm text-muted-foreground">Loading…</p>
  if (bound.isError) return <p className="text-sm text-destructive">Failed to load bindings.</p>
  if (bound.rows.length === 0)
    return <p className="text-sm text-muted-foreground">No releases bound to this target.</p>

  return (
    <div className="space-y-2">
      {bound.rows.map((row) => {
        const relName = row.binding?.spec.releaseRef.name
        return (
          <ReleaseRow
            key={`${row.namespace}/${row.name}`}
            namespace={row.namespace}
            name={row.name}
            binding={row.binding}
            forbidden={row.forbidden}
            failed={row.failed}
            renderTask={relName ? bound.renderTasks.get(`${row.namespace}/${relName}`) : undefined}
            renderTasksLoading={bound.renderTasksLoading}
            renderTasksFailed={bound.renderTasksError}
          />
        )
      })}
    </div>
  )
}
