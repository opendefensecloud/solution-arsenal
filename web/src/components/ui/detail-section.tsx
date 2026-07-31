// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react'

export function DetailSection({
  title,
  count,
  children,
}: {
  title: string
  count?: number
  children: ReactNode
}) {
  return (
    <div>
      <h3 className="mb-3 text-sm font-semibold text-foreground">
        {title}
        {count !== undefined && ` (${count})`}
      </h3>
      {children}
    </div>
  )
}
