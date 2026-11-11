// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useNavigate } from '@tanstack/react-router'
import { DeleteResourceDialog } from '@/components/delete-resource-dialog'
import { useReleaseDelete } from '@/api/mutations'

export function DeleteReleaseDialog({
  open,
  onOpenChange,
  namespace,
  name,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  namespace: string
  name: string
}) {
  const navigate = useNavigate()
  return (
    <DeleteResourceDialog
      open={open}
      onOpenChange={onOpenChange}
      kind="Release"
      namespace={namespace}
      name={name}
      useDelete={useReleaseDelete}
      onDeleted={() => navigate({ to: '/releases' })}
    >
      <p className="text-sm text-muted-foreground">
        A Release still referenced by a <strong className="text-foreground">Profile</strong> or{' '}
        <strong className="text-foreground">ReleaseBinding</strong> stays in deletion until those
        references are removed.
      </p>
    </DeleteResourceDialog>
  )
}
