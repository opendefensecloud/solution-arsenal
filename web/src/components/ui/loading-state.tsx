// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { LucideIcon } from 'lucide-react'

interface LoadingStateProps {
  icon: LucideIcon
  label: string
}

export function LoadingState({ icon: Icon, label }: LoadingStateProps) {
  return (
    <div className="flex items-center gap-2 text-muted-foreground" role="status" aria-live="polite">
      <Icon className="h-4 w-4 animate-pulse" />
      {label}
    </div>
  )
}
