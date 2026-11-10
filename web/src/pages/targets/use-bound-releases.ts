// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo } from 'react'
import { useQueries, useQuery } from '@tanstack/react-query'
import { isForbiddenError } from '@/api/client'
import { releaseBindingQueries, renderTaskQueries } from '@/api/queries'
import type { ReleaseBinding, RenderTask, Target } from '@/api/types'

export interface BoundReleaseRow {
  // namespace and name identify the ReleaseBinding.
  namespace: string
  name: string
  binding?: ReleaseBinding
  forbidden: boolean
  failed: boolean
}

export interface BoundReleases {
  rows: BoundReleaseRow[]
  isLoading: boolean
  isError: boolean
  // renderTasks maps "<release namespace>/<release name>" to its RenderTask.
  renderTasks: Map<string, RenderTask>
  renderTasksLoading: boolean
  renderTasksError: boolean
}

// useBoundReleases lists a Target's bindings. They come from the Target's
// resolvedBindings, which include bindings from other namespaces, each read
// with the viewer's own credentials. Without that list, as before the first
// reconcile after an upgrade, it falls back to the Target namespace's listing.
export function useBoundReleases(
  namespace: string,
  name: string,
  target: Target | undefined
): BoundReleases {
  const resolved = target?.status?.resolvedBindings
  const listFallback = target !== undefined && resolved === undefined

  const listQ = useQuery({ ...releaseBindingQueries.list(namespace), enabled: listFallback })
  const bindingQs = useQueries({
    queries: (resolved ?? []).map((entry) => ({
      ...releaseBindingQueries.detail(entry.namespace, entry.name),
      // A viewer without read access won't gain it by asking again.
      retry: (failures: number, err: unknown) => !isForbiddenError(err) && failures < 3,
    })),
  })
  const renderTasksQ = useQuery(renderTaskQueries.list(namespace))

  const rows = useMemo<BoundReleaseRow[]>(() => {
    if (resolved) {
      return resolved.map((entry, i) => ({
        namespace: entry.namespace,
        name: entry.name,
        binding: bindingQs[i]?.data,
        forbidden: isForbiddenError(bindingQs[i]?.error),
        failed: !!bindingQs[i]?.isError && !isForbiddenError(bindingQs[i]?.error),
      }))
    }

    return (listQ.data?.items ?? [])
      .filter(
        (b) =>
          b.spec.targetRef.name === name &&
          (b.spec.targetRef.namespace ?? b.metadata.namespace) === namespace
      )
      .map((b) => ({
        namespace: b.metadata.namespace,
        name: b.metadata.name,
        binding: b,
        forbidden: false,
        failed: false,
      }))
  }, [resolved, bindingQs, listQ.data, name, namespace])

  // A release RenderTask's repository is <target ns>/<release ns>/release-<name>.
  const renderTasks = useMemo(() => {
    const m = new Map<string, RenderTask>()
    for (const rt of renderTasksQ.data?.items ?? []) {
      if (rt.spec.ownerName !== name) continue
      const [, relNamespace, chart] = (rt.spec.repository ?? '').split('/')
      if (relNamespace && chart?.startsWith('release-'))
        m.set(`${relNamespace}/${chart.slice('release-'.length)}`, rt)
    }
    return m
  }, [renderTasksQ.data, name])

  return {
    rows,
    isLoading: target === undefined || (listFallback && listQ.isLoading),
    isError: listFallback && listQ.isError,
    renderTasks,
    renderTasksLoading: renderTasksQ.isLoading,
    renderTasksError: renderTasksQ.isError,
  }
}
