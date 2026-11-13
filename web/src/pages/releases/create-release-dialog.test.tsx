// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { CreateReleaseDialog } from './create-release-dialog'

const { useReleaseCreate, mutate, cvList } = vi.hoisted(() => {
  const mutate = vi.fn()
  const cvList = vi.fn((ns: string) => ({
    queryKey: ['componentversions', ns],
    queryFn: async () => {
      if (ns === 'locked') throw { status: 403, message: 'forbidden' }
      return { items: [{ metadata: { name: `cv-${ns}` } }] }
    },
    retry: false,
  }))
  return { mutate, cvList, useReleaseCreate: vi.fn(() => ({ mutate, isPending: false })) }
})
vi.mock('@/api/mutations', () => ({ useReleaseCreate }))
vi.mock('@/components/ui/toast', () => ({ useToast: () => ({ toast: vi.fn() }) }))
vi.mock('@/api/queries', () => ({
  namespaceQueries: {
    list: () => ({
      queryKey: ['namespaces'],
      queryFn: async () => ({
        items: ['team-a', 'shared', 'locked'].map((name) => ({ metadata: { name } })),
      }),
    }),
  },
  componentVersionQueries: { list: cvList },
}))

function options() {
  const select = screen.getByLabelText(/^component version$/i)
  return [...select.querySelectorAll('option')].map((o) => o.value)
}

const expectOption = (name: string) => waitFor(() => expect(options()).toContain(name))

const set = (label: RegExp | string, value: string) =>
  fireEvent.change(screen.getByLabelText(label), { target: { value } })

function wrap(children: ReactNode) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{children}</QueryClientProvider>)
}

describe('CreateReleaseDialog', () => {
  afterEach(() => {
    cleanup()
    mutate.mockClear()
    cvList.mockClear()
  })

  it('blocks submit when name and component version are empty', () => {
    wrap(<CreateReleaseDialog open onOpenChange={() => {}} namespace="default" />)
    fireEvent.click(screen.getByRole('button', { name: /create/i }))
    expect(screen.getByText(/name is required/i)).toBeInTheDocument()
    expect(screen.getByText(/component version is required/i)).toBeInTheDocument()
    expect(mutate).not.toHaveBeenCalled()
  })

  it('requires picking a namespace in the all-namespaces view', async () => {
    wrap(<CreateReleaseDialog open onOpenChange={() => {}} namespace={null} />)
    expect(await screen.findByRole('option', { name: 'team-a' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /create/i }))
    expect(screen.getByText(/namespace is required/i)).toBeInTheDocument()
  })

  it('creates in the picked namespace', async () => {
    wrap(<CreateReleaseDialog open onOpenChange={() => {}} namespace={null} />)
    await screen.findByRole('option', { name: 'team-a' })
    fireEvent.change(screen.getByLabelText('Namespace'), { target: { value: 'team-a' } })
    expect(useReleaseCreate).toHaveBeenLastCalledWith('team-a')
    await expectOption('team-a/cv-team-a')
    set('Name', 'r1')
    set(/^component version$/i, 'team-a/cv-team-a')
    fireEvent.click(screen.getByRole('button', { name: /create/i }))
    expect(mutate).toHaveBeenCalledWith(
      { metadata: { name: 'r1' }, spec: { componentVersionRef: { name: 'cv-team-a' } } },
      expect.any(Object)
    )
  })

  it('lists component versions of every accessible namespace, own namespace first', async () => {
    wrap(<CreateReleaseDialog open onOpenChange={() => {}} namespace="default" />)
    await expectOption('shared/cv-shared')
    const groups = [...document.querySelectorAll('optgroup')].map((g) => g.label)
    expect(groups).toEqual(['default', 'shared', 'team-a'])
    expect(cvList).toHaveBeenCalledWith('locked')
    expect(screen.getByText(/ReferenceGrant/)).toBeInTheDocument()
    expect(screen.queryByLabelText(/component version namespace/i)).not.toBeInTheDocument()
  })

  it('references a component version in another namespace', async () => {
    wrap(<CreateReleaseDialog open onOpenChange={() => {}} namespace="default" />)
    await expectOption('shared/cv-shared')
    set('Name', 'r1')
    set(/^component version$/i, 'shared/cv-shared')
    fireEvent.click(screen.getByRole('button', { name: /create/i }))
    expect(mutate).toHaveBeenCalledWith(
      {
        metadata: { name: 'r1' },
        spec: { componentVersionRef: { name: 'cv-shared', namespace: 'shared' } },
      },
      expect.any(Object)
    )
  })

  it('submits every spec field', async () => {
    wrap(<CreateReleaseDialog open onOpenChange={() => {}} namespace="default" />)
    await expectOption('default/cv-default')
    set('Name', 'r1')
    set(/^component version$/i, 'default/cv-default')
    set(/target namespace/i, 'apps')
    set(/unique name/i, 'kyverno')
    set(/priority/i, '7')
    set(/failed job ttl/i, '120')
    set(/anti-affinity/i, 'matchLabels:\n  app: x')
    set(/^values/i, 'replicas: 2')
    fireEvent.click(screen.getByRole('button', { name: /create/i }))
    expect(mutate).toHaveBeenCalledWith(
      {
        metadata: { name: 'r1' },
        spec: {
          componentVersionRef: { name: 'cv-default' },
          targetNamespace: 'apps',
          uniqueName: 'kyverno',
          antiAffinity: { matchLabels: { app: 'x' } },
          values: { replicas: 2 },
          failedJobTTL: 120,
          priority: 7,
        },
      },
      expect.any(Object)
    )
  })
})
