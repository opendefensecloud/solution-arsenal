// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { parse, stringify } from 'yaml'
import { inputCls } from '@/components/ui/controls'

export function parseYaml(
  text: string
): { ok: true; value: unknown } | { ok: false; error: string } {
  if (text.trim() === '') return { ok: true, value: undefined }
  try {
    return { ok: true, value: parse(text) }
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : 'Invalid YAML' }
  }
}

export function stringifyYaml(value: unknown): string {
  return value !== undefined ? stringify(value) : ''
}

export function YamlField({
  id,
  value,
  onChange,
  error,
}: {
  id?: string
  value: string
  onChange: (s: string) => void
  error?: string
}) {
  return (
    <div>
      <textarea
        id={id}
        className={`${inputCls} h-32 font-mono`}
        placeholder={'key: value'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      {error && <p className="mt-1 text-xs text-destructive">{error}</p>}
    </div>
  )
}
