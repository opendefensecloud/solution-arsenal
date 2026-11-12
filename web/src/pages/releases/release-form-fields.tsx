// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { useId, type ReactNode } from 'react'
import { useQueries, useQuery } from '@tanstack/react-query'
import { componentVersionQueries, namespaceQueries } from '@/api/queries'
import { inputCls } from '@/components/ui/controls'
import { YamlField } from '@/components/yaml-field'
import type { ReleaseForm, ReleaseFormErrors } from './release-form'

export function Field({
  label,
  hint,
  error,
  children,
}: {
  label: string
  hint?: ReactNode
  error?: string
  children: (id: string) => ReactNode
}) {
  const id = useId()
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs font-medium text-muted-foreground">
        {label}
      </label>
      {children(id)}
      {hint && <p className="mt-1 text-xs text-muted-foreground">{hint}</p>}
      {error && <p className="mt-1 text-xs text-destructive">{error}</p>}
    </div>
  )
}

/**
 * Inputs for every Release.spec field. `namespace` is the Release's namespace
 * ('' while none is picked). ComponentVersions are offered from every
 * namespace the user can list them in, grouped by namespace.
 */
export function ReleaseFormFields({
  namespace,
  form,
  onChange,
  errors,
  uniqueNameReadOnly = false,
}: {
  namespace: string
  form: ReleaseForm
  onChange: (patch: Partial<ReleaseForm>) => void
  errors: ReleaseFormErrors
  uniqueNameReadOnly?: boolean
}) {
  const nsQ = useQuery(namespaceQueries.list())
  const namespaces = [
    ...new Set([namespace, ...(nsQ.data?.items ?? []).map((n) => n.metadata.name)]),
  ].filter((ns) => ns !== '')
  // Listed per namespace: a cluster-wide list needs cluster-scoped RBAC, and
  // namespaces whose ComponentVersions cannot be listed are skipped.
  const cvQs = useQueries({
    queries: namespaces.map((ns) => ({ ...componentVersionQueries.list(ns), retry: false })),
  })
  const groups = namespaces
    .map((ns, i) => ({ ns, names: (cvQs[i]?.data?.items ?? []).map((cv) => cv.metadata.name) }))
    .filter((g) => g.names.length > 0)
    .sort((a, b) => (a.ns === namespace ? -1 : b.ns === namespace ? 1 : a.ns.localeCompare(b.ns)))

  const selectedNs = form.componentVersionNamespace || namespace
  const selected = form.componentVersion === '' ? '' : `${selectedNs}/${form.componentVersion}`
  const listed = groups.some((g) => g.ns === selectedNs && g.names.includes(form.componentVersion))
  const select = (value: string) => {
    const [ns = '', name = ''] = value.split('/')
    onChange({
      componentVersion: name,
      componentVersionNamespace: ns === namespace ? '' : ns,
    })
  }

  return (
    <>
      <Field
        label="Component version"
        hint="A ComponentVersion from another namespace can only be used if that namespace has a ReferenceGrant for this namespace."
        error={errors.componentVersion}
      >
        {(id) => (
          <select
            id={id}
            className={inputCls}
            value={selected}
            disabled={namespace === ''}
            onChange={(e) => select(e.target.value)}
          >
            <option value="">
              {namespace === '' ? 'Select a namespace first…' : 'Select a component version…'}
            </option>
            {selected !== '' && !listed && (
              <option value={selected}>
                {selectedNs === namespace ? form.componentVersion : selected}
              </option>
            )}
            {groups.map((g) => (
              <optgroup key={g.ns} label={g.ns}>
                {g.names.map((name) => (
                  <option key={name} value={`${g.ns}/${name}`}>
                    {name}
                  </option>
                ))}
              </optgroup>
            ))}
          </select>
        )}
      </Field>

      <Field label="Target namespace (optional)" error={errors.targetNamespace}>
        {(id) => (
          <input
            id={id}
            className={inputCls}
            value={form.targetNamespace}
            onChange={(e) => onChange({ targetNamespace: e.target.value })}
          />
        )}
      </Field>

      {!uniqueNameReadOnly && (
        <Field
          label="Unique name (optional)"
          hint="Deduplicates Releases on a Target. Defaults to the component name. Cannot be changed later."
        >
          {(id) => (
            <input
              id={id}
              className={inputCls}
              value={form.uniqueName}
              onChange={(e) => onChange({ uniqueName: e.target.value })}
            />
          )}
        </Field>
      )}

      <div className="grid grid-cols-2 gap-4">
        <Field label="Priority (optional)" error={errors.priority}>
          {(id) => (
            <input
              id={id}
              type="text"
              className={inputCls}
              placeholder="0"
              value={form.priority}
              onChange={(e) => onChange({ priority: e.target.value })}
            />
          )}
        </Field>
        <Field label="Failed job TTL in seconds (optional)" error={errors.failedJobTTL}>
          {(id) => (
            <input
              id={id}
              type="text"
              inputMode="numeric"
              className={inputCls}
              placeholder="3600"
              value={form.failedJobTTL}
              onChange={(e) => onChange({ failedJobTTL: e.target.value })}
            />
          )}
        </Field>
      </div>

      <Field label="Anti-affinity label selector (YAML, optional)">
        {(id) => (
          <YamlField
            id={id}
            value={form.antiAffinity}
            onChange={(v) => onChange({ antiAffinity: v })}
            error={errors.antiAffinity}
          />
        )}
      </Field>

      <Field label="Values (YAML, optional)">
        {(id) => (
          <YamlField
            id={id}
            value={form.values}
            onChange={(v) => onChange({ values: v })}
            error={errors.values}
          />
        )}
      </Field>
    </>
  )
}
