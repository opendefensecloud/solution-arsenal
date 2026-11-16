// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react'
import { Dialog } from '@/components/ui/dialog'
import { useToast } from '@/components/ui/toast'
import { useReleaseCreate } from '@/api/mutations'
import { isApiError } from '@/api/client'
import { parseK8sName } from '@/lib/k8s-name'
import { inputCls, btnPrimary, btnGhost } from '@/components/ui/controls'
import { NamespacePickerField } from '@/components/namespace-picker-field'
import { Field, ReleaseFormFields } from './release-form-fields'
import {
  emptyReleaseForm,
  parseReleaseForm,
  type ReleaseForm,
  type ReleaseFormErrors,
} from './release-form'

export function CreateReleaseDialog({
  open,
  onOpenChange,
  namespace,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  namespace: string | null
}) {
  const { toast } = useToast()
  const [pickedNs, setPickedNs] = useState('')
  const ns = namespace ?? pickedNs
  const create = useReleaseCreate(ns)

  const [name, setName] = useState('')
  const [form, setForm] = useState<ReleaseForm>(emptyReleaseForm)
  const [nsErr, setNsErr] = useState<string>()
  const [nameErr, setNameErr] = useState<string>()
  const [errors, setErrors] = useState<ReleaseFormErrors>({})

  const reset = () => {
    setPickedNs('')
    setNsErr(undefined)
    setName('')
    setForm(emptyReleaseForm)
    setNameErr(undefined)
    setErrors({})
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) reset()
    onOpenChange(nextOpen)
  }

  const submit = () => {
    let bad = false
    if (ns === '') {
      setNsErr('Namespace is required.')
      bad = true
    } else setNsErr(undefined)

    const parsedName = parseK8sName(name)
    if (!parsedName.ok) {
      setNameErr(parsedName.error)
      bad = true
    } else setNameErr(undefined)

    const parsed = parseReleaseForm(form)
    setErrors(parsed.ok ? {} : parsed.errors)

    if (bad || !parsedName.ok || !parsed.ok) return

    create.mutate(
      { metadata: { name: parsedName.value }, spec: parsed.spec },
      {
        onSuccess: () => {
          toast(`Release "${parsedName.value}" created.`)
          reset()
          onOpenChange(false)
        },
        onError: (e) => toast(isApiError(e) ? e.message : 'Failed to create release.', 'error'),
      }
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title="New Release"
      footer={
        <>
          <button type="button" className={btnGhost} onClick={() => handleOpenChange(false)}>
            Cancel
          </button>
          <button type="button" className={btnPrimary} disabled={create.isPending} onClick={submit}>
            Create
          </button>
        </>
      }
    >
      <NamespacePickerField
        fixed={namespace}
        value={pickedNs}
        onChange={(v) => {
          setPickedNs(v)
          setForm((f) => ({ ...f, componentVersion: '', componentVersionNamespace: '' }))
        }}
        error={nsErr}
        disabled={create.isPending}
      />

      <Field label="Name" error={nameErr}>
        {(id) => (
          <input
            id={id}
            className={inputCls}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        )}
      </Field>

      <ReleaseFormFields
        namespace={ns}
        form={form}
        onChange={(patch) => setForm((f) => ({ ...f, ...patch }))}
        errors={errors}
      />
    </Dialog>
  )
}
