// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { stringify } from 'yaml'

export function YamlBlock({ value }: { value?: unknown }) {
  if (value === undefined || value === null) return null
  if (typeof value === 'object' && Object.keys(value as object).length === 0) return null
  return (
    <pre className="overflow-x-auto rounded-lg border border-border bg-muted/30 p-3 text-xs font-mono text-foreground">
      {stringify(value)}
    </pre>
  )
}
