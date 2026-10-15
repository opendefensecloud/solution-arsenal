// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { CreateTargetDialog } from './create-target-dialog'

const { useTargetCreate, useReleaseBindingCreate } = vi.hoisted(() => ({
  useTargetCreate: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useReleaseBindingCreate: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
}))
vi.mock('@/api/mutations', () => ({ useTargetCreate, useReleaseBindingCreate }))
vi.mock('@/components/ui/toast', () => ({ useToast: () => ({ toast: vi.fn() }) }))
vi.mock('@/api/queries', () => ({
  namespaceQueries: {
    list: () => ({
      queryKey: ['namespaces'],
      queryFn: async () => ({
        items: [{ metadata: { name: 'team-a' } }, { metadata: { name: 'team-b' } }],
      }),
    }),
  },
  registryQueries: {
    list: () => ({
      queryKey: ['registries'],
      queryFn: async () => ({ items: [{ metadata: { name: 'registry-a' } }] }),
    }),
  },
  releaseQueries: {
    list: () => ({
      queryKey: ['releases'],
      queryFn: async () => ({ items: [{ metadata: { name: 'release-a' } }] }),
    }),
  },
}))

function wrap(children: ReactNode) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{children}</QueryClientProvider>)
}

describe('CreateTargetDialog', () => {
  afterEach(() => {
    cleanup()
  })

  it('blocks submit when name is empty', () => {
    wrap(<CreateTargetDialog open onOpenChange={() => {}} namespace="default" />)
    fireEvent.click(screen.getByRole('button', { name: /create/i }))
    expect(screen.getByText(/name is required/i)).toBeInTheDocument()
  })

  it('requires picking a namespace in the all-namespaces view', async () => {
    wrap(<CreateTargetDialog open onOpenChange={() => {}} namespace={null} />)
    expect(screen.queryByText(/creating in namespace/i)).not.toBeInTheDocument()
    expect(await screen.findByRole('option', { name: 'team-a' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /create/i }))
    expect(screen.getByText(/namespace is required/i)).toBeInTheDocument()
  })

  it('creates and binds in the picked namespace', async () => {
    wrap(<CreateTargetDialog open onOpenChange={() => {}} namespace={null} />)
    await screen.findByRole('option', { name: 'team-a' })
    fireEvent.change(screen.getByLabelText('Namespace'), { target: { value: 'team-a' } })
    expect(useTargetCreate).toHaveBeenLastCalledWith('team-a')
    expect(useReleaseBindingCreate).toHaveBeenLastCalledWith('team-a')
  })

  it('clears registry and releases when the picked namespace changes', async () => {
    wrap(<CreateTargetDialog open onOpenChange={() => {}} namespace={null} />)
    await screen.findByRole('option', { name: 'team-b' })
    const ns = screen.getByLabelText('Namespace')
    fireEvent.change(ns, { target: { value: 'team-a' } })
    // The dialog renders into a portal and the label has no htmlFor, so query the document.
    const registry = document.querySelector('input[list="target-registries"]') as HTMLInputElement
    fireEvent.change(registry, { target: { value: 'registry-a' } })
    fireEvent.click(await screen.findByRole('checkbox', { name: 'release-a' }))

    fireEvent.change(ns, { target: { value: 'team-b' } })
    expect(registry.value).toBe('')
    expect(screen.getByRole('checkbox', { name: 'release-a' })).not.toBeChecked()
  })
})
