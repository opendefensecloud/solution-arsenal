// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Package } from 'lucide-react'
import { isForbiddenError } from '@/api/client'
import { componentVersionQueries, releaseQueries } from '@/api/queries'
import type { ReleaseBinding, RenderTask } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { StatusDot } from '@/components/ui/status-dot'
import { cn, renderTaskPhase } from '@/lib/utils'
import { releaseFlags, releaseRowStatus } from './release-status'

export interface ReleaseRowProps {
  // namespace and name identify the ReleaseBinding.
  namespace: string
  name: string
  binding?: ReleaseBinding
  // forbidden means the viewer may not read the binding.
  forbidden?: boolean
  failed?: boolean
  renderTask?: RenderTask
  renderTasksLoading?: boolean
  renderTasksFailed?: boolean
}

// ReleaseRow is one binding in a Target's Bound Releases, with one status.
export function ReleaseRow({
  namespace,
  name,
  binding,
  forbidden,
  failed,
  renderTask,
  renderTasksLoading,
  renderTasksFailed,
}: ReleaseRowProps) {
  const relName = binding?.spec.releaseRef.name
  const phase = renderTask?.status?.conditions?.length
    ? renderTaskPhase(renderTask.status.conditions)
    : undefined
  const status = releaseRowStatus(binding, phase)
  const flags = releaseFlags(binding)
  const version = useComponentVersionTag(namespace, relName)

  let statusView = null
  if (forbidden) {
    statusView = <span className="text-xs text-muted-foreground">No access to details</span>
  } else if (failed) {
    statusView = <span className="text-xs text-destructive">Failed to load binding</span>
  } else if (renderTasksLoading) {
    statusView = <span className="text-xs text-muted-foreground">…</span>
  } else if (renderTasksFailed && !status) {
    statusView = <span className="text-xs text-destructive">Failed to load phase</span>
  } else if (status) {
    statusView = (
      <>
        <StatusDot color={status.color} />
        <span
          className={cn(
            'text-xs text-muted-foreground',
            status.color === 'danger' && 'text-destructive'
          )}
        >
          {status.label}
        </span>
      </>
    )
  }

  return (
    <div
      data-testid={`release-row-${namespace}/${name}`}
      className={cn(
        'rounded-lg border border-border bg-card px-4 py-3',
        status?.greyed && 'opacity-60'
      )}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Package className="h-4 w-4 text-muted-foreground" />
          {relName ? (
            <>
              <Link
                to="/releases/$namespace/$name"
                params={{ namespace, name: relName }}
                className="text-sm font-medium text-foreground hover:text-primary transition-colors"
              >
                {relName}
              </Link>
              <Link
                to="/releasebindings/$namespace/$name"
                params={{ namespace, name }}
                className="text-xs text-muted-foreground hover:text-primary transition-colors"
              >
                View binding
              </Link>
              {version && <span className="text-xs text-muted-foreground">{version}</span>}
            </>
          ) : (
            <span className="text-sm font-medium text-foreground">{`${namespace}/${name}`}</span>
          )}
        </div>
        <div className="flex items-center gap-1.5">
          {flags.map((flag) => (
            <Badge key={flag.label} variant="warning" title={flag.detail || undefined}>
              {flag.label}
            </Badge>
          ))}
          {statusView}
        </div>
      </div>
      {status?.detail && <p className="mt-2 text-xs text-muted-foreground">{status.detail}</p>}
    </div>
  )
}

const retryUnlessForbidden = (failures: number, err: unknown) =>
  !isForbiddenError(err) && failures < 3

function useComponentVersionTag(namespace: string, release: string | undefined) {
  const releaseQ = useQuery({
    ...releaseQueries.detail(namespace, release ?? ''),
    enabled: !!release,
    retry: retryUnlessForbidden,
  })
  const ref = releaseQ.data?.spec.componentVersionRef
  const cvQ = useQuery({
    ...componentVersionQueries.detail(ref?.namespace ?? namespace, ref?.name ?? ''),
    enabled: !!ref,
    retry: retryUnlessForbidden,
  })

  return cvQ.data?.spec.tag
}
