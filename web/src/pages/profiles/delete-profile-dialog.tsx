// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useNavigate } from '@tanstack/react-router'
import { DeleteResourceDialog } from '@/components/delete-resource-dialog'
import { useProfileDelete } from '@/api/mutations'

export function DeleteProfileDialog({
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
      kind="Profile"
      namespace={namespace}
      name={name}
      useDelete={useProfileDelete}
      onDeleted={() => navigate({ to: '/profiles' })}
    >
      <p className="text-sm text-muted-foreground">
        Deleting this Profile also removes every{' '}
        <strong className="text-foreground">ReleaseBinding</strong> it owns. This cascade cannot be
        undone.
      </p>
    </DeleteResourceDialog>
  )
}
