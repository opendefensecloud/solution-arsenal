// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

interface ErrorStateProps {
  message: string
}

export function ErrorState({ message }: ErrorStateProps) {
  return (
    <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4">
      <p className="text-sm text-destructive">{message}</p>
    </div>
  )
}
