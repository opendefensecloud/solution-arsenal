// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { Condition } from '@/api/types'
import { getCondition } from '@/lib/utils'

interface ReleasesReadyStatProps {
  conditions?: Condition[]
}

// ReleasesReadyStat is the Target page's Releases ready card value: the
// ReleasesDeployed message, which counts every binding, including those from
// namespaces the viewer may not read.
export function ReleasesReadyStat({ conditions }: ReleasesReadyStatProps) {
  const deployed = getCondition(conditions, 'ReleasesDeployed')
  if (!deployed) return <span>—</span>

  return <span title={deployed.reason}>{deployed.message || deployed.reason}</span>
}
