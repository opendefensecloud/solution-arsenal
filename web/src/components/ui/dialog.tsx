// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react'
import * as RD from '@radix-ui/react-dialog'
import { X } from 'lucide-react'

export function Dialog({
  open,
  onOpenChange,
  title,
  children,
  footer,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  children: ReactNode
  footer?: ReactNode
}) {
  return (
    <RD.Root open={open} onOpenChange={onOpenChange}>
      <RD.Portal>
        <RD.Overlay className="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm" />
        <RD.Content
          aria-describedby={undefined}
          className="fixed left-1/2 top-1/2 z-50 w-[92vw] max-w-lg -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 shadow-lg focus:outline-none"
        >
          <div className="mb-4 flex items-center justify-between">
            <RD.Title className="text-lg font-semibold text-foreground">{title}</RD.Title>
            <RD.Close className="text-muted-foreground hover:text-foreground" aria-label="Close">
              <X className="h-4 w-4" />
            </RD.Close>
          </div>
          <div className="space-y-4">{children}</div>
          {footer && <div className="mt-5 flex justify-end gap-2">{footer}</div>}
        </RD.Content>
      </RD.Portal>
    </RD.Root>
  )
}
