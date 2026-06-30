// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { LucideIcon } from 'lucide-react'

interface EmptyStateProps {
  icon?: LucideIcon
  message: string
}

export function EmptyState({ icon: Icon, message }: EmptyStateProps) {
  return (
    <div className={`rounded-lg border-2 border-dashed border-border text-center ${Icon ? 'py-12' : 'py-8'}`}>
      {Icon && <Icon className="mx-auto mb-3 h-10 w-10 text-muted-foreground/40" />}
      <p className={Icon ? 'text-muted-foreground' : 'text-sm text-muted-foreground'}>{message}</p>
    </div>
  )
}
