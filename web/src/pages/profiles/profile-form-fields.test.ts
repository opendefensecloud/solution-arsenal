// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect } from 'vitest'
import { parseUserdata, stringifyUserdata } from './profile-form-fields'

describe('parseUserdata', () => {
  it('treats blank input as no userdata', () => {
    expect(parseUserdata('   ')).toEqual({ ok: true, value: undefined })
  })

  it('parses YAML', () => {
    expect(parseUserdata('a: 1\nb:\n  - x\n  - y')).toEqual({
      ok: true,
      value: { a: 1, b: ['x', 'y'] },
    })
  })

  it('parses JSON too (YAML is a superset)', () => {
    expect(parseUserdata('{"a": 1}')).toEqual({ ok: true, value: { a: 1 } })
  })

  it('reports an error on malformed YAML', () => {
    const r = parseUserdata('a: [1, 2')
    expect(r.ok).toBe(false)
  })

  it('round-trips a value back to YAML for editing', () => {
    expect(parseUserdata(stringifyUserdata({ a: 1 }))).toEqual({ ok: true, value: { a: 1 } })
  })
})
