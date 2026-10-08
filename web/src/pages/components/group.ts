// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { ComponentVersion } from '@/api/types'

export interface ComponentGroup {
  namespace: string
  name: string
  registry: string
  repository: string
  versionCount: number
  creationTimestamp: string
}

/** Groups ComponentVersions into components by namespace and component name. */
export function groupComponents(cvs: ComponentVersion[]): ComponentGroup[] {
  const groups = new Map<string, ComponentGroup>()
  for (const cv of cvs) {
    const name = cv.spec.componentName
    if (!name) continue
    const key = `${cv.metadata.namespace}/${name}`
    const g = groups.get(key)
    if (g) {
      g.versionCount++
      if (cv.metadata.creationTimestamp < g.creationTimestamp)
        g.creationTimestamp = cv.metadata.creationTimestamp
    } else {
      groups.set(key, {
        namespace: cv.metadata.namespace,
        name,
        registry: cv.spec.registry,
        repository: cv.spec.repository,
        versionCount: 1,
        creationTimestamp: cv.metadata.creationTimestamp,
      })
    }
  }
  return [...groups.values()]
}
