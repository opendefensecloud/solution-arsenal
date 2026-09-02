// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import { BackButton } from './back-button'
import { Badge } from './badge'

interface DetailHeaderProps {
  icon: LucideIcon
  title: string
  namespace: string
  subtitle?: ReactNode
  badges?: ReactNode
  actions?: ReactNode
  backLabel: string
  onBack: () => void
}

export function DetailHeader({
  icon: Icon,
  title,
  namespace,
  subtitle,
  badges,
  actions,
  backLabel,
  onBack,
}: DetailHeaderProps) {
  return (
    <div className="space-y-4">
      <BackButton label={backLabel} onClick={onBack} />
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
            <Icon className="h-6 w-6 text-primary" />
          </div>
          <div>
            <div className="flex items-center gap-2 flex-wrap">
              <h1 className="text-2xl font-bold text-foreground">{title}</h1>
              <Badge variant="secondary">{namespace}</Badge>
              {badges}
            </div>
            {subtitle && <p className="text-sm text-muted-foreground font-mono">{subtitle}</p>}
          </div>
        </div>
        {actions && <div className="flex items-center gap-2">{actions}</div>}
      </div>
    </div>
  )
}
