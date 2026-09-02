// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react'
import { Dialog } from '@/components/ui/dialog'
import { useToast } from '@/components/ui/toast'
import { useProfileUpdate } from '@/api/mutations'
import { isApiError } from '@/api/client'
import type { Profile } from '@/api/types'
import { LabelEditor, YamlField, parseUserdata, stringifyUserdata } from './profile-form-fields'
import { btnPrimary, btnGhost } from '@/components/ui/controls'

export function EditProfileDialog({
  open,
  onOpenChange,
  profile,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  profile: Profile
}) {
  const { toast } = useToast()
  const update = useProfileUpdate(profile.metadata.namespace, profile.metadata.name)

  const [labels, setLabels] = useState<Record<string, string>>(
    profile.spec.targetSelector?.matchLabels ?? {}
  )
  const [userdataText, setUserdataText] = useState(stringifyUserdata(profile.spec.userdata))
  const [jsonErr, setJsonErr] = useState<string>()

  const submit = () => {
    const parsed = parseUserdata(userdataText)
    if (!parsed.ok) {
      setJsonErr(parsed.error)
      return
    }
    setJsonErr(undefined)

    update.mutate(
      {
        spec: {
          targetSelector: { matchLabels: labels },
          userdata: parsed.value ?? null,
        },
      },
      {
        onSuccess: () => {
          toast('Profile updated.')
          onOpenChange(false)
        },
        onError: (e) => toast(isApiError(e) ? e.message : 'Failed to update profile.', 'error'),
      }
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={`Edit ${profile.metadata.name}`}
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
        Release <span className="font-mono text-foreground">{profile.spec.releaseRef.name}</span> is
        immutable.
      </div>
      <div>
        <label className="mb-1 block text-xs font-medium text-muted-foreground">
          Target selector (match labels)
        </label>
        <LabelEditor value={labels} onChange={setLabels} />
      </div>
      <div>
        <label className="mb-1 block text-xs font-medium text-muted-foreground">
          Userdata (YAML)
        </label>
        <YamlField value={userdataText} onChange={setUserdataText} error={jsonErr} />
      </div>
    </Dialog>
  )
}
