// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { Release } from '@/api/types'

const { mutate } = vi.hoisted(() => ({ mutate: vi.fn() }))
vi.mock('@/api/mutations', () => ({ useReleaseUpdate: () => ({ mutate, isPending: false }) }))
vi.mock('@/components/ui/toast', () => ({ useToast: () => ({ toast: vi.fn() }) }))
vi.mock('@/api/queries', () => ({
  namespaceQueries: {
    list: () => ({ queryKey: ['namespaces'], queryFn: async () => ({ items: [] }) }),
  },
  componentVersionQueries: {
    list: (ns: string) => ({
      queryKey: ['componentversions', ns],
      queryFn: async () => ({
        items: [{ metadata: { name: 'cv1' } }, { metadata: { name: 'cv2' } }],
      }),
    }),
  },
}))

import { EditReleaseDialog } from './edit-release-dialog'

const release = {
  metadata: { name: 'r1', namespace: 'default', creationTimestamp: '' },
  spec: {
    componentVersionRef: { name: 'cv1' },
    targetNamespace: 'apps',
    uniqueName: 'kyverno',
    priority: 4,
  },
} as Release

function renderDialog() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <EditReleaseDialog open onOpenChange={() => {}} release={release} />
    </QueryClientProvider>
  )
}

describe('EditReleaseDialog', () => {
  afterEach(() => {
    cleanup()
    mutate.mockClear()
  })

  it('shows uniqueName read-only', () => {
    renderDialog()
    expect(screen.getByText('kyverno')).toBeInTheDocument()
    expect(screen.getByText(/is immutable/i)).toBeInTheDocument()
    expect(screen.queryByLabelText(/unique name/i)).not.toBeInTheDocument()
  })

  it('keeps the current component version selectable when it is not listed', async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <EditReleaseDialog
          open
          onOpenChange={() => {}}
          release={{ ...release, spec: { ...release.spec, componentVersionRef: { name: 'gone' } } }}
        />
      </QueryClientProvider>
    )
    await screen.findByRole('option', { name: 'cv2' })
    expect(screen.getByRole('combobox', { name: /^component version$/i })).toHaveValue(
      'default/gone'
    )
  })

  it('removes cleared optional fields and updates the component version', async () => {
    renderDialog()
    await screen.findByRole('option', { name: 'cv2' })
    fireEvent.change(screen.getByLabelText(/^component version$/i), {
      target: { value: 'default/cv2' },
    })
    fireEvent.change(screen.getByLabelText(/target namespace/i), { target: { value: '' } })
    fireEvent.change(screen.getByLabelText(/failed job ttl/i), { target: { value: '30' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))
    expect(mutate).toHaveBeenCalledWith(
      {
        spec: {
          componentVersionRef: { name: 'cv2' },
          targetNamespace: null,
          failedJobTTL: 30,
          priority: 4,
        },
      },
      expect.any(Object)
    )
  })

  it('does not save invalid YAML', () => {
    renderDialog()
    fireEvent.change(screen.getByLabelText(/^values/i), { target: { value: 'a: [' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))
    expect(mutate).not.toHaveBeenCalled()
  })
})
