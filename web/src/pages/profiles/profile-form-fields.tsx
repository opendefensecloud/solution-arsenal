// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { Plus, X } from 'lucide-react'
import { parse as parseYaml, stringify as stringifyYaml } from 'yaml'
import { inputCls } from '@/components/ui/controls'

export function LabelEditor({
  value,
  onChange,
}: {
  value: Record<string, string>
  onChange: (v: Record<string, string>) => void
}) {
  const rows = Object.entries(value)
  const setRow = (i: number, k: string, v: string) => {
    const next = rows.map((r, idx) => (idx === i ? [k, v] : r)) as [string, string][]
    onChange(Object.fromEntries(next.filter(([key, val]) => key !== '' || val !== '')))
  }
  const add = () => onChange({ ...value, '': '' })
  const remove = (key: string) => {
    const next = { ...value }
    delete next[key]
    onChange(next)
  }

  return (
    <div className="space-y-2">
      {rows.map(([k, v], i) => (
        <div key={i} className="flex items-center gap-2">
          <input
            className={inputCls}
            placeholder="key"
            value={k}
            onChange={(e) => setRow(i, e.target.value, v)}
          />
          <input
            className={inputCls}
            placeholder="value"
            value={v}
            onChange={(e) => setRow(i, k, e.target.value)}
          />
          <button
            type="button"
            aria-label="Remove label"
            onClick={() => remove(k)}
            className="text-muted-foreground hover:text-destructive"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={add}
        className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
      >
        <Plus className="h-3.5 w-3.5" /> Add label
      </button>
    </div>
  )
}

export function parseUserdata(
  text: string
): { ok: true; value: unknown } | { ok: false; error: string } {
  if (text.trim() === '') return { ok: true, value: undefined }
  try {
    return { ok: true, value: parseYaml(text) }
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : 'Invalid YAML' }
  }
}

export function stringifyUserdata(value: unknown): string {
  return value !== undefined ? stringifyYaml(value) : ''
}

export function YamlField({
  value,
  onChange,
  error,
}: {
  value: string
  onChange: (s: string) => void
  error?: string
}) {
  return (
    <div>
      <textarea
        className={`${inputCls} h-32 font-mono`}
        placeholder={'key: value'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      {error && <p className="mt-1 text-xs text-destructive">{error}</p>}
    </div>
  )
}
