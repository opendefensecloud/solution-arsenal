// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { CreateProfileDialog } from './create-profile-dialog'

const { useProfileCreate } = vi.hoisted(() => ({
  useProfileCreate: vi.fn(() => ({ mutate: vi.fn(), isPending: false })),
}))
vi.mock('@/api/mutations', () => ({ useProfileCreate }))
vi.mock('@/components/ui/toast', () => ({ useToast: () => ({ toast: vi.fn() }) }))
vi.mock('@/api/queries', () => ({
  namespaceQueries: {
    list: () => ({
      queryKey: ['namespaces'],
      queryFn: async () => ({ items: [{ metadata: { name: 'team-a' } }] }),
    }),
  },
  releaseQueries: {
    list: () => ({ queryKey: ['releases'], queryFn: async () => ({ items: [] }) }),
  },
}))

function wrap(children: ReactNode) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{children}</QueryClientProvider>)
}

describe('CreateProfileDialog', () => {
  afterEach(() => {
    cleanup()
  })

  it('blocks submit when name is empty', () => {
    wrap(<CreateProfileDialog open onOpenChange={() => {}} namespace="default" />)
    fireEvent.click(screen.getByRole('button', { name: /create/i }))
    expect(screen.getByText(/name is required/i)).toBeInTheDocument()
  })

  it('requires picking a namespace in the all-namespaces view', async () => {
    wrap(<CreateProfileDialog open onOpenChange={() => {}} namespace={null} />)
    expect(screen.queryByText(/creating in namespace/i)).not.toBeInTheDocument()
    expect(await screen.findByRole('option', { name: 'team-a' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /create/i }))
    expect(screen.getByText(/namespace is required/i)).toBeInTheDocument()
  })

  it('creates in the picked namespace', async () => {
    wrap(<CreateProfileDialog open onOpenChange={() => {}} namespace={null} />)
    await screen.findByRole('option', { name: 'team-a' })
    expect(screen.getByRole('option', { name: /select a namespace first/i })).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('Namespace'), { target: { value: 'team-a' } })
    expect(useProfileCreate).toHaveBeenLastCalledWith('team-a')
    expect(screen.getByRole('option', { name: /select a release/i })).toBeInTheDocument()
  })
})
