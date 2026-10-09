// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { ComponentVersion } from '@/api/types'

export interface ComponentGroup {
  namespace: string
  name: string
  registries: string[]
  repositories: string[]
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
      if (!g.registries.includes(cv.spec.registry)) g.registries.push(cv.spec.registry)
      if (!g.repositories.includes(cv.spec.repository)) g.repositories.push(cv.spec.repository)
      if (cv.metadata.creationTimestamp < g.creationTimestamp)
        g.creationTimestamp = cv.metadata.creationTimestamp
    } else {
      groups.set(key, {
        namespace: cv.metadata.namespace,
        name,
        registries: [cv.spec.registry],
        repositories: [cv.spec.repository],
        versionCount: 1,
        creationTimestamp: cv.metadata.creationTimestamp,
      })
    }
  }
  return [...groups.values()]
}
