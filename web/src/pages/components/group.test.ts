// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest'
import type { ComponentVersion } from '@/api/types'
import { groupComponents } from './group'

const cv = (ns: string, name: string, componentName: string, ts = '2026-01-01T00:00:00Z') =>
  ({
    metadata: { name, namespace: ns, creationTimestamp: ts },
    spec: {
      componentName,
      scheme: 'https',
      registry: 'r.io',
      repository: `p/${componentName}`,
      tag: name,
    },
  }) as unknown as ComponentVersion

describe('groupComponents', () => {
  it('groups versions by namespace and component name', () => {
    const groups = groupComponents([
      cv('a', 'arc-v1', 'opendefense.cloud/arc'),
      cv('a', 'arc-v2', 'opendefense.cloud/arc'),
      cv('b', 'arc-v1', 'opendefense.cloud/arc'),
    ])
    expect(groups).toHaveLength(2)
    expect(groups.find((g) => g.namespace === 'a')?.versionCount).toBe(2)
    expect(groups.find((g) => g.namespace === 'b')?.versionCount).toBe(1)
    expect(groups[0]).toMatchObject({
      name: 'opendefense.cloud/arc',
      registry: 'r.io',
      repository: 'p/opendefense.cloud/arc',
    })
  })

  it('skips versions without a component name', () => {
    expect(groupComponents([cv('a', 'legacy', '')])).toEqual([])
  })

  it('uses the oldest version creation time as the component age', () => {
    const [g] = groupComponents([
      cv('a', 'v2', 'x', '2026-02-01T00:00:00Z'),
      cv('a', 'v1', 'x', '2026-01-01T00:00:00Z'),
    ])
    expect(g.creationTimestamp).toBe('2026-01-01T00:00:00Z')
  })
})
