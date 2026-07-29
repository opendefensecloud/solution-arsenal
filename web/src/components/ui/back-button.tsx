// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { ArrowLeft } from 'lucide-react'

interface BackButtonProps {
  label: string
  onClick: () => void
}

export function BackButton({ label, onClick }: BackButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
    >
      <ArrowLeft className="h-4 w-4" />
      {label}
    </button>
  )
}
