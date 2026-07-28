// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react'
import { Dialog } from '@/components/ui/dialog'
import { useToast } from '@/components/ui/toast'
import { btnDanger, btnGhost } from '@/components/ui/controls'
import { isApiError } from '@/api/client'

type DeleteHook = (namespace: string) => {
  mutate: (name: string, opts?: { onSuccess?: () => void; onError?: (e: unknown) => void }) => void
  isPending: boolean
}

// onDeleted runs after a successful delete. It's a callback so routing stays type-safe at the call site.
export function DeleteResourceDialog({
  open,
  onOpenChange,
  kind,
  namespace,
  name,
  useDelete,
  onDeleted,
  children,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  kind: string
  namespace: string
  name: string
  useDelete: DeleteHook
  onDeleted?: () => void
  children?: ReactNode
}) {
  const { toast } = useToast()
  const del = useDelete(namespace)

  const confirm = () =>
    del.mutate(name, {
      onSuccess: () => {
        toast(`${kind} "${name}" deleted.`)
        onOpenChange(false)
        onDeleted?.()
      },
      onError: (e) =>
        toast(isApiError(e) ? e.message : `Failed to delete ${kind.toLowerCase()}.`, 'error'),
    })

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={`Delete ${name}?`}
      footer={
        <>
          <button type="button" className={btnGhost} onClick={() => onOpenChange(false)}>
            Cancel
          </button>
          <button type="button" className={btnDanger} disabled={del.isPending} onClick={confirm}>
            Delete
          </button>
        </>
      }
    >
      {children}
    </Dialog>
  )
}
