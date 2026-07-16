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
import { ReleaseSelect } from './release-select'
import { inputCls, btnPrimary, btnGhost } from '@/components/ui/controls'

const K8S_NAME_REGEX = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/

export function CreateTargetDialog({
  open,
  onOpenChange,
  namespace,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  namespace: string
}) {
  const { toast } = useToast()
  const create = useTargetCreate(namespace)
  const bind = useReleaseBindingCreate(namespace)
  const registriesQ = useQuery(registryQueries.list(namespace))
  const registries = registriesQ.data?.items ?? []

  const [name, setName] = useState('')
  const [registryRef, setRegistryRef] = useState('')
  const [userdataText, setUserdataText] = useState('')
  const [releases, setReleases] = useState<Set<string>>(new Set())
  const [nameErr, setNameErr] = useState<string>()
  const [registryErr, setRegistryErr] = useState<string>()
  const [jsonErr, setJsonErr] = useState<string>()

  const reset = () => {
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
    if (name.trim() === '') {
      setNameErr('Name is required.')
      bad = true
    } else if (!K8S_NAME_REGEX.test(name) || name.length > 63) {
      setNameErr('Label must be ≤63 and contain only lowercase alphanumeric characters and "-".')
      bad = true
    } else setNameErr(undefined)

    if (registryRef.trim() === '') {
      setRegistryErr('Render registry is required.')
      bad = true
    } else setRegistryErr(undefined)

    const parsed = parseUserdata(userdataText)
    if (!parsed.ok) {
      setJsonErr(parsed.error)
      bad = true
    } else setJsonErr(undefined)

    if (bad || !parsed.ok) return

    try {
      await create.mutateAsync({
        metadata: { name },
        spec: {
          renderRegistryRef: { name: registryRef.trim() },
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
        await bind.mutateAsync({ target: name, release })
      } catch {
        failed.push(release)
      }
    }

    if (failed.length > 0) {
      toast(`Target created, but binding failed for: ${failed.join(', ')}`, 'error')
    } else {
      toast(`Target "${name}" created.`)
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
      <div className="rounded-md bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
        Creating in namespace <span className="font-mono text-foreground">{namespace}</span>
      </div>

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
        <ReleaseSelect namespace={namespace} value={releases} onChange={setReleases} />
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
