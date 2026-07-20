// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useNavigate } from '@tanstack/react-router'
import { DeleteResourceDialog } from '@/components/delete-resource-dialog'
import { useTargetDelete } from '@/api/mutations'

export function DeleteTargetDialog({
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
      kind="Target"
      namespace={namespace}
      name={name}
      useDelete={useTargetDelete}
      onDeleted={() => navigate({ to: '/targets' })}
    >
      <p className="text-sm text-muted-foreground">
        Deleting this Target is a <strong className="text-foreground">cascading</strong> operation!
      </p>
    </DeleteResourceDialog>
  )
}
