// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { api } from './client'
import { bindingName, useProfileCreate, useProfileDelete, useProfileUpdate } from './mutations'

vi.mock('./client', () => ({
  api: { post: vi.fn(), jsonPatch: vi.fn(), delete: vi.fn() },
  isApiError: () => true,
}))

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

// A wrapper bound to a caller-supplied QueryClient, so a test can seed the
// cache and then inspect it after a mutation.
function wrapperFor(qc: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  )
}

beforeEach(() => vi.clearAllMocks())

describe('useProfileCreate', () => {
  it('POSTs to the namespace profiles path', async () => {
    ;(api.post as ReturnType<typeof vi.fn>).mockResolvedValue({})
    const { result } = renderHook(() => useProfileCreate('default'), { wrapper })
    result.current.mutate({
      metadata: { name: 'p1' },
      spec: { releaseRef: { name: 'r1' } },
    })
    await waitFor(() => expect(api.post).toHaveBeenCalled())
    expect(api.post).toHaveBeenCalledWith('/namespaces/default/profiles', expect.any(Object))
  })
})

describe('bindingName', () => {
  it('does not collide for pairs whose dash-join is ambiguous', () => {
    expect(bindingName('a-b', 'c')).not.toEqual(bindingName('a', 'b-c'))
  })
})

describe('useProfileUpdate', () => {
  it('sends a JSON Patch that replaces userdata wholesale', async () => {
    ;(api.jsonPatch as ReturnType<typeof vi.fn>).mockResolvedValue({})
    const { result } = renderHook(() => useProfileUpdate('default', 'p1'), { wrapper })
    // User dropped the "b" key in the editor — only "a" remains.
    result.current.mutate({ spec: { userdata: { a: '1' } } })
    await waitFor(() => expect(api.jsonPatch).toHaveBeenCalled())
    expect(api.jsonPatch).toHaveBeenCalledWith('/namespaces/default/profiles/p1', [
      { op: 'add', path: '/spec/userdata', value: { a: '1' } },
    ])
  })
})

describe('useProfileDelete', () => {
  it('DELETEs the named profile', async () => {
    ;(api.delete as ReturnType<typeof vi.fn>).mockResolvedValue(undefined)
    const { result } = renderHook(() => useProfileDelete('default'), { wrapper })
    result.current.mutate('p1')
    await waitFor(() => expect(api.delete).toHaveBeenCalledWith('/namespaces/default/profiles/p1'))
  })

  it('optimistically removes the row and rolls back when the delete fails', async () => {
    ;(api.delete as ReturnType<typeof vi.fn>).mockRejectedValue({ status: 500, message: 'boom' })
    const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    // list query key mirrors api/queries.ts: ['profiles', nsKey(namespace)].
    qc.setQueryData(['profiles', 'default'], {
      items: [
        { metadata: { name: 'p1', namespace: 'default' } },
        { metadata: { name: 'p2', namespace: 'default' } },
      ],
    })
    const { result } = renderHook(() => useProfileDelete('default'), { wrapper: wrapperFor(qc) })

    result.current.mutate('p1')

    await waitFor(() => expect(result.current.isError).toBe(true))
    const list = qc.getQueryData(['profiles', 'default']) as {
      items: { metadata: { name: string } }[]
    }
    expect(list.items.map((p) => p.metadata.name)).toEqual(['p1', 'p2'])
  })

  it('optimistic delete only touches rows in the mutation namespace', async () => {
    ;(api.delete as ReturnType<typeof vi.fn>).mockResolvedValue(undefined)
    const qc = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
    qc.setQueryData(['profiles', 'default'], {
      items: [{ metadata: { name: 'p1', namespace: 'default' } }],
    })
    qc.setQueryData(['profiles', 'other'], {
      items: [{ metadata: { name: 'p1', namespace: 'other' } }],
    })
    const { result } = renderHook(() => useProfileDelete('default'), { wrapper: wrapperFor(qc) })

    result.current.mutate('p1')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const def = qc.getQueryData(['profiles', 'default']) as { items: unknown[] }
    const other = qc.getQueryData(['profiles', 'other']) as {
      items: { metadata: { name: string } }[]
    }
    expect(def.items).toEqual([])
    // Same-named profile in another namespace must be untouched.
    expect(other.items.map((p) => p.metadata.name)).toEqual(['p1'])
  })
})
