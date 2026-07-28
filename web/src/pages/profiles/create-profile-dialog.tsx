// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Dialog } from '@/components/ui/dialog'
import { useToast } from '@/components/ui/toast'
import { useProfileCreate } from '@/api/mutations'
import { releaseQueries } from '@/api/queries'
import { isApiError } from '@/api/client'
import { LabelEditor, YamlField, parseUserdata } from './profile-form-fields'
import { inputCls, btnPrimary, btnGhost } from '@/components/ui/controls'

const K8S_NAME_REGEX = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/

export function CreateProfileDialog({
  open,
  onOpenChange,
  namespace,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  namespace: string
}) {
  const { toast } = useToast()
  const create = useProfileCreate(namespace)
  const releasesQ = useQuery(releaseQueries.list(namespace))
  const releases = releasesQ.data?.items ?? []

  const [name, setName] = useState('')
  const [releaseRef, setReleaseRef] = useState('')
  const [labels, setLabels] = useState<Record<string, string>>({})
  const [userdataText, setUserdataText] = useState('')
  const [nameErr, setNameErr] = useState<string>()
  const [releaseErr, setReleaseErr] = useState<string>()
  const [jsonErr, setJsonErr] = useState<string>()

  const reset = () => {
    setName('')
    setReleaseRef('')
    setLabels({})
    setUserdataText('')
    setNameErr(undefined)
    setReleaseErr(undefined)
    setJsonErr(undefined)
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) reset()
    onOpenChange(nextOpen)
  }

  const submit = () => {
    let bad = false
    if (name.trim() === '') {
      setNameErr('Name is required.')
      bad = true
    } else if (!K8S_NAME_REGEX.test(name) || name.length > 63) {
      setNameErr('Label must be ≤63 and contain only lowercase alphanumeric characters and "-".')
      bad = true
    } else setNameErr(undefined)

    if (releaseRef === '') {
      setReleaseErr('Release is required.')
      bad = true
    } else setReleaseErr(undefined)

    const parsed = parseUserdata(userdataText)
    if (!parsed.ok) {
      setJsonErr(parsed.error)
      bad = true
    } else setJsonErr(undefined)

    if (bad || !parsed.ok) return

    create.mutate(
      {
        metadata: { name },
        spec: {
          releaseRef: { name: releaseRef },
          ...(Object.keys(labels).length ? { targetSelector: { matchLabels: labels } } : {}),
          ...(parsed.value !== undefined ? { userdata: parsed.value } : {}),
        },
      },
      {
        onSuccess: () => {
          toast(`Profile "${name}" created.`)
          reset()
          onOpenChange(false)
        },
        onError: (e) => toast(isApiError(e) ? e.message : 'Failed to create profile.', 'error'),
      }
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={handleOpenChange}
      title="New Profile"
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
      <div className="rounded-md bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
        Creating in namespace <span className="font-mono text-foreground">{namespace}</span>
      </div>

      <div>
        <label className="mb-1 block text-xs font-medium text-muted-foreground">Name</label>
        <input className={inputCls} value={name} onChange={(e) => setName(e.target.value)} />
        {nameErr && <p className="mt-1 text-xs text-destructive">{nameErr}</p>}
      </div>

      <div>
        <label className="mb-1 block text-xs font-medium text-muted-foreground">Release</label>
        <select
          className={inputCls}
          value={releaseRef}
          onChange={(e) => setReleaseRef(e.target.value)}
        >
          <option value="">Select a release…</option>
          {releases.map((r) => (
            <option key={r.metadata.name} value={r.metadata.name}>
              {r.metadata.name}
            </option>
          ))}
        </select>
        {releaseErr && <p className="mt-1 text-xs text-destructive">{releaseErr}</p>}
      </div>

      <div>
        <label className="mb-1 block text-xs font-medium text-muted-foreground">
          Target selector (match labels)
        </label>
        <LabelEditor value={labels} onChange={setLabels} />
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
