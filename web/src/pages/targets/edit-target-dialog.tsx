// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Dialog } from '@/components/ui/dialog'
import { useToast } from '@/components/ui/toast'
import { useTargetUpdate, useReleaseBindingCreate, useReleaseBindingDelete } from '@/api/mutations'
import { releaseBindingQueries } from '@/api/queries'
import { isApiError } from '@/api/client'
import type { Target } from '@/api/types'
import { YamlField, parseUserdata, stringifyUserdata } from '@/pages/profiles/profile-form-fields'
import { ReleaseSelect } from './release-select'
import { btnPrimary, btnGhost } from '@/components/ui/controls'

export function EditTargetDialog({
  open,
  onOpenChange,
  target,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  target: Target
}) {
  const { toast } = useToast()
  const ns = target.metadata.namespace
  const name = target.metadata.name
  const update = useTargetUpdate(ns, name)
  const bind = useReleaseBindingCreate(ns)
  const unbind = useReleaseBindingDelete(ns)

  const bindingsQ = useQuery(releaseBindingQueries.list(ns))
  const bindingByRelease = useMemo(() => {
    const m = new Map<string, string>()
    for (const b of bindingsQ.data?.items ?? []) {
      if (
        b.spec.targetRef.name === name &&
        (b.spec.targetNamespace ?? b.metadata.namespace) === ns
      ) {
        m.set(b.spec.releaseRef.name, b.metadata.name)
      }
    }
    return m
  }, [bindingsQ.data, name, ns])

  const [userdataText, setUserdataText] = useState(stringifyUserdata(target.spec.userdata))
  const [jsonErr, setJsonErr] = useState<string>()
  const [releases, setReleases] = useState<Set<string> | null>(null)
  const selected = releases ?? new Set(bindingByRelease.keys())

  const submit = async () => {
    const parsed = parseUserdata(userdataText)
    if (!parsed.ok) {
      setJsonErr(parsed.error)
      return
    }
    setJsonErr(undefined)

    try {
      await update.mutateAsync({ spec: { userdata: parsed.value ?? null } })
    } catch (e) {
      toast(isApiError(e) ? e.message : 'Failed to update target.', 'error')
      return
    }

    const existing = new Set(bindingByRelease.keys())
    const toAdd = [...selected].filter((r) => !existing.has(r))
    const toRemove = [...existing].filter((r) => !selected.has(r))
    const failed: string[] = []
    for (const release of toAdd) {
      try {
        await bind.mutateAsync({ target: name, release })
      } catch {
        failed.push(release)
      }
    }
    for (const release of toRemove) {
      try {
        await unbind.mutateAsync(bindingByRelease.get(release)!)
      } catch {
        failed.push(release)
      }
    }

    if (failed.length > 0) {
      // Keep the dialog open so the user can retry the still-unresolved bindings.
      toast(`Target saved, but some bindings failed: ${failed.join(', ')}`, 'error')
      return
    }
    toast('Target updated.')
    onOpenChange(false)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={`Edit ${name}`}
      footer={
        <>
          <button type="button" className={btnGhost} onClick={() => onOpenChange(false)}>
            Cancel
          </button>
          <button
            type="button"
            className={btnPrimary}
            disabled={update.isPending || bind.isPending || unbind.isPending}
            onClick={submit}
          >
            Save
          </button>
        </>
      }
    >
      <div className="rounded-md bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
        Render registry{' '}
        <span className="font-mono text-foreground">{target.spec.renderRegistryRef.name}</span> is
        immutable.
      </div>
      <div>
        <label className="mb-1 block text-xs font-medium text-muted-foreground">
          Bound releases
        </label>
        <ReleaseSelect namespace={ns} value={selected} onChange={setReleases} />
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
