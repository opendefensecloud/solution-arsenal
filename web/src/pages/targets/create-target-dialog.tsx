// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Dialog } from '@/components/ui/dialog'
import { useToast } from '@/components/ui/toast'
import { useTargetCreate, useReleaseBindingCreate } from '@/api/mutations'
import { registryQueries } from '@/api/queries'
import { isApiError } from '@/api/client'
import { YamlField, parseUserdata } from '@/pages/profiles/profile-form-fields'
import { parseK8sName } from '@/lib/k8s-name'
import { ReleaseSelect } from './release-select'
import { inputCls, btnPrimary, btnGhost } from '@/components/ui/controls'
import { NamespacePickerField } from '@/components/namespace-picker-field'

export function CreateTargetDialog({
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
  const create = useTargetCreate(ns)
  const bind = useReleaseBindingCreate(ns)
  const registriesQ = useQuery({ ...registryQueries.list(ns), enabled: ns !== '' })
  const registries = registriesQ.data?.items ?? []

  const [name, setName] = useState('')
  const [registryRef, setRegistryRef] = useState('')
  const [userdataText, setUserdataText] = useState('')
  const [releases, setReleases] = useState<Set<string>>(new Set())
  const [nsErr, setNsErr] = useState<string>()
  const [nameErr, setNameErr] = useState<string>()
  const [registryErr, setRegistryErr] = useState<string>()
  const [jsonErr, setJsonErr] = useState<string>()

  const reset = () => {
    setPickedNs('')
    setNsErr(undefined)
    setName('')
    setRegistryRef('')
    setUserdataText('')
    setReleases(new Set())
    setNameErr(undefined)
    setRegistryErr(undefined)
    setJsonErr(undefined)
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) reset()
    onOpenChange(nextOpen)
  }

  const submit = async () => {
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

    const parsedRegistryRef = parseK8sName(registryRef, 'Render registry is required.')
    if (!parsedRegistryRef.ok) {
      setRegistryErr(parsedRegistryRef.error)
      bad = true
    } else setRegistryErr(undefined)

    const parsed = parseUserdata(userdataText)
    if (!parsed.ok) {
      setJsonErr(parsed.error)
      bad = true
    } else setJsonErr(undefined)

    if (bad || !parsedName.ok || !parsedRegistryRef.ok || !parsed.ok) return

    try {
      await create.mutateAsync({
        metadata: { name: parsedName.value },
        spec: {
          renderRegistryRef: { name: parsedRegistryRef.value },
          ...(parsed.value !== undefined ? { userdata: parsed.value } : {}),
        },
      })
    } catch (e) {
      toast(isApiError(e) ? e.message : 'Failed to create target.', 'error')
      return
    }

    const failed: string[] = []
    for (const release of releases) {
      try {
        await bind.mutateAsync({ target: parsedName.value, release })
      } catch {
        failed.push(release)
      }
    }

    if (failed.length > 0) {
      toast(`Target created, but binding failed for: ${failed.join(', ')}`, 'error')
    } else {
      toast(`Target "${parsedName.value}" created.`)
    }
    reset()
    onOpenChange(false)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title="New Target"
      footer={
        <>
          <button type="button" className={btnGhost} onClick={() => handleOpenChange(false)}>
            Cancel
          </button>
          <button
            type="button"
            className={btnPrimary}
            disabled={create.isPending || bind.isPending}
            onClick={submit}
          >
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
          setRegistryRef('')
          setReleases(new Set())
        }}
        error={nsErr}
        disabled={create.isPending || bind.isPending}
      />

      <div>
        <label className="mb-1 block text-xs font-medium text-muted-foreground">Name</label>
        <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} />
        {nameErr && <p className="mt-1 text-xs text-destructive">{nameErr}</p>}
      </div>

      <div>
        <label className="mb-1 block text-xs font-medium text-muted-foreground">
          Render registry
        </label>
        <input
          className={inputCls}
          list="target-registries"
          value={registryRef}
          onChange={(e) => setRegistryRef(e.target.value)}
        />
        <datalist id="target-registries">
          {registries.map((r) => (
            <option key={r.metadata.name} value={r.metadata.name} />
          ))}
        </datalist>
        {registryErr && <p className="mt-1 text-xs text-destructive">{registryErr}</p>}
      </div>

      <div>
        <label className="mb-1 block text-xs font-medium text-muted-foreground">
          Bind releases (optional)
        </label>
        <ReleaseSelect namespace={ns} value={releases} onChange={setReleases} />
      </div>

      <div>
        <label className="mb-1 block text-xs font-medium text-muted-foreground">
          Userdata (YAML, optional)
        </label>
        <YamlField value={userdataText} onChange={setUserdataText} error={jsonErr} />
      </div>
    </Dialog>
  )
}
