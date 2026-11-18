// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import { parseYaml, stringifyYaml } from './yaml-field'

describe('parseYaml', () => {
  it('treats blank input as unset', () => {
    expect(parseYaml('   ')).toEqual({ ok: true, value: undefined })
  })

  it('parses YAML', () => {
    expect(parseYaml('a: 1\nb:\n  - x\n  - y')).toEqual({
      ok: true,
      value: { a: 1, b: ['x', 'y'] },
    })
  })

  it('parses JSON too (YAML is a superset)', () => {
    expect(parseYaml('{"a": 1}')).toEqual({ ok: true, value: { a: 1 } })
  })

  it('reports an error on malformed YAML', () => {
    const r = parseYaml('a: [1, 2')
    expect(r.ok).toBe(false)
  })

  it('round-trips a value back to YAML for editing', () => {
    expect(parseYaml(stringifyYaml({ a: 1 }))).toEqual({ ok: true, value: { a: 1 } })
  })
})
