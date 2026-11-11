// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react'
import { Dialog } from '@/components/ui/dialog'
import { useToast } from '@/components/ui/toast'
import { useReleaseUpdate } from '@/api/mutations'
import { isApiError } from '@/api/client'
import type { Release } from '@/api/types'
import { btnPrimary, btnGhost } from '@/components/ui/controls'
import { ReleaseFormFields } from './release-form-fields'
import {
  buildReleasePatch,
  parseReleaseForm,
  releaseToForm,
  type ReleaseForm,
  type ReleaseFormErrors,
} from './release-form'

export function EditReleaseDialog({
  open,
  onOpenChange,
  release,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  release: Release
}) {
  const { toast } = useToast()
  const update = useReleaseUpdate(release.metadata.namespace, release.metadata.name)

  const [form, setForm] = useState<ReleaseForm>(() => releaseToForm(release))
  const [errors, setErrors] = useState<ReleaseFormErrors>({})

  const submit = () => {
    const parsed = parseReleaseForm(form)
    if (!parsed.ok) {
      setErrors(parsed.errors)
      return
    }
    setErrors({})

    update.mutate(buildReleasePatch(release, parsed.spec), {
      onSuccess: () => {
        toast('Release updated.')
        onOpenChange(false)
      },
      onError: (e) => toast(isApiError(e) ? e.message : 'Failed to update release.', 'error'),
    })
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={`Edit ${release.metadata.name}`}
      footer={
        <>
          <button type="button" className={btnGhost} onClick={() => onOpenChange(false)}>
            Cancel
          </button>
          <button type="button" className={btnPrimary} disabled={update.isPending} onClick={submit}>
            Save
          </button>
        </>
      }
    >
      <div className="rounded-md bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
        Unique name <span className="font-mono text-foreground">{form.uniqueName || '—'}</span> is
        immutable.
      </div>
      <ReleaseFormFields
        namespace={release.metadata.namespace}
        form={form}
        onChange={(patch) => setForm((f) => ({ ...f, ...patch }))}
        errors={errors}
        uniqueNameReadOnly
      />
    </Dialog>
  )
}
