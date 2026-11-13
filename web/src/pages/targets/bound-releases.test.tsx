// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, cleanup, within } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import type { ReleaseBinding, RenderTask, ResolvedBinding, Target } from '@/api/types'
import { BoundReleaseList } from './bound-releases'
import { useBoundReleases } from './use-bound-releases'

const { bindingFn, listFn, renderTasksFn, releaseFn, componentVersionFn } = vi.hoisted(() => ({
  bindingFn: vi.fn(),
  listFn: vi.fn(),
  renderTasksFn: vi.fn(),
  releaseFn: vi.fn(),
  componentVersionFn: vi.fn(),
}))

vi.mock('@/api/queries', () => ({
  releaseQueries: {
    detail: (namespace: string, name: string) => ({
      queryKey: ['releases', namespace, name],
      queryFn: () => releaseFn(namespace, name),
    }),
  },
  componentVersionQueries: {
    detail: (namespace: string, name: string) => ({
      queryKey: ['componentversions', namespace, name],
      queryFn: () => componentVersionFn(namespace, name),
    }),
  },
  releaseBindingQueries: {
    detail: (namespace: string, name: string) => ({
      queryKey: ['releasebindings', namespace, name],
      queryFn: () => bindingFn(namespace, name),
    }),
    list: (namespace: string) => ({
      queryKey: ['releasebindings', namespace, 'list'],
      queryFn: () => listFn(namespace),
    }),
  },
  renderTaskQueries: {
    list: (namespace: string) => ({
      queryKey: ['rendertasks', namespace],
      queryFn: () => renderTasksFn(namespace),
    }),
  },
}))

vi.mock('@tanstack/react-router', () => ({
  Link: ({
    to,
    params,
    children,
  }: {
    to: string
    params: { namespace: string; name: string }
    children: ReactNode
  }) => (
    <a href={to.replace('$namespace', params.namespace).replace('$name', params.name)}>
      {children}
    </a>
  ),
}))

const NOW = '2026-11-09T12:00:00Z'

const target = (resolvedBindings?: ResolvedBinding[]): Target => ({
  metadata: { name: 'edge-01', namespace: 'team-a', creationTimestamp: NOW },
  spec: { renderRegistryRef: { name: 'reg' } },
  status: { resolvedBindings },
})

const binding = (
  namespace: string,
  name: string,
  release: string,
  targetName = 'edge-01'
): ReleaseBinding => ({
  metadata: { name, namespace, creationTimestamp: NOW },
  spec: { targetRef: { name: targetName, namespace: 'team-a' }, releaseRef: { name: release } },
})

const renderTask = (releaseNamespace: string, release: string, condition: string): RenderTask =>
  ({
    metadata: {
      name: `rt-${releaseNamespace}-${release}`,
      namespace: 'team-a',
      creationTimestamp: NOW,
    },
    spec: { ownerName: 'edge-01', repository: `team-a/${releaseNamespace}/release-${release}` },
    status: {
      conditions: [
        {
          type: condition,
          status: 'True',
          reason: condition,
          message: '',
          lastTransitionTime: NOW,
        },
      ],
    },
  }) as RenderTask

function Harness({ target: t }: { target: Target }) {
  const bound = useBoundReleases('team-a', 'edge-01', t)
  return <BoundReleaseList bound={bound} />
}

function renderList(t: Target) {
  // The app's default client, which retries failed queries.
  const qc = new QueryClient()
  return render(
    <QueryClientProvider client={qc}>
      <Harness target={t} />
    </QueryClientProvider>
  )
}

describe('BoundReleaseList', () => {
  beforeEach(() => {
    // Rows that don't test the version label get none.
    releaseFn.mockRejectedValue({ status: 404, message: 'not found' })
  })

  afterEach(() => {
    cleanup()
    bindingFn.mockReset()
    listFn.mockReset()
    renderTasksFn.mockReset()
    releaseFn.mockReset()
    componentVersionFn.mockReset()
  })

  it('labels a rolled back and drifted release, with its component version', async () => {
    const b = binding('team-a', 'rb-1', 'rel-a')
    b.status = {
      conditions: [
        { type: 'Deployed', status: 'True', reason: 'Ready', message: '', lastTransitionTime: NOW },
        {
          type: 'RolledBack',
          status: 'True',
          reason: 'RollbackSucceeded',
          message:
            'Helm rollback to previous release team-a/app.v2 with chart app@0.0.1-aa succeeded',
          lastTransitionTime: NOW,
        },
        {
          type: 'Drifted',
          status: 'True',
          reason: 'DriftDetected',
          message: '',
          lastTransitionTime: NOW,
        },
      ],
    }
    bindingFn.mockResolvedValue(b)
    renderTasksFn.mockResolvedValue({ items: [] })
    releaseFn.mockResolvedValue({
      metadata: { name: 'rel-a', namespace: 'team-a', creationTimestamp: NOW },
      spec: { componentVersionRef: { name: 'app-v1-9-0' } },
    })
    componentVersionFn.mockResolvedValue({
      metadata: { name: 'app-v1-9-0', namespace: 'team-a', creationTimestamp: NOW },
      spec: { componentName: 'app', scheme: 'oci', registry: 'r', repository: 'app', tag: '1.9.0' },
    })

    renderList(target([{ namespace: 'team-a', name: 'rb-1', uniqueName: 'app' }]))

    const row = await screen.findByTestId('release-row-team-a/rb-1')
    expect(await within(row).findByText('Ready')).toBeInTheDocument()
    expect(within(row).getByText(/^Rolled back: .*0\.0\.1-aa/)).toBeInTheDocument()
    expect(within(row).getByText('Drifted')).toBeInTheDocument()
    expect(await within(row).findByText('1.9.0')).toBeInTheDocument()
    expect(componentVersionFn).toHaveBeenCalledWith('team-a', 'app-v1-9-0')
  })

  it('renders one row per resolved binding, including other namespaces', async () => {
    bindingFn.mockImplementation((ns: string, name: string) =>
      Promise.resolve(binding(ns, name, ns === 'team-a' ? 'rel-a' : 'rel-b'))
    )
    renderTasksFn.mockResolvedValue({ items: [] })

    renderList(
      target([
        { namespace: 'shared', name: 'rb-2', uniqueName: 'b' },
        { namespace: 'team-a', name: 'rb-1', uniqueName: 'a' },
      ])
    )

    expect(await screen.findByRole('link', { name: 'rel-b' })).toHaveAttribute(
      'href',
      '/releases/shared/rel-b'
    )
    expect(screen.getByRole('link', { name: 'rel-a' })).toHaveAttribute(
      'href',
      '/releases/team-a/rel-a'
    )
    expect(
      within(screen.getByTestId('release-row-shared/rb-2')).getByRole('link', {
        name: 'View binding',
      })
    ).toHaveAttribute('href', '/releasebindings/shared/rb-2')
    expect(listFn).not.toHaveBeenCalled()
  })

  it('shows a forbidden binding by name without retrying, and the others still render', async () => {
    bindingFn.mockImplementation((ns: string, name: string) =>
      ns === 'shared'
        ? Promise.reject({ status: 403, message: 'forbidden' })
        : Promise.resolve(binding(ns, name, 'rel-a'))
    )
    renderTasksFn.mockResolvedValue({ items: [] })

    renderList(
      target([
        { namespace: 'shared', name: 'rb-2', uniqueName: 'b' },
        { namespace: 'team-a', name: 'rb-1', uniqueName: 'a' },
      ])
    )

    const row = await screen.findByTestId('release-row-shared/rb-2')
    expect(await within(row).findByText('No access to details')).toBeInTheDocument()
    expect(within(row).getByText('shared/rb-2')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'rel-a' })).toBeInTheDocument()
    expect(bindingFn.mock.calls.filter(([ns]) => ns === 'shared')).toHaveLength(1)
  })

  it('falls back to the namespace listing without resolvedBindings', async () => {
    listFn.mockResolvedValue({
      items: [
        binding('team-a', 'rb-1', 'rel-a'),
        binding('team-a', 'rb-other', 'rel-x', 'edge-02'),
      ],
    })
    renderTasksFn.mockResolvedValue({ items: [] })

    renderList(target(undefined))

    expect(await screen.findByRole('link', { name: 'rel-a' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'rel-x' })).toBeNull()
    expect(bindingFn).not.toHaveBeenCalled()
  })

  it('shows each same-named release its own render phase', async () => {
    bindingFn.mockImplementation((ns: string, name: string) =>
      Promise.resolve(binding(ns, name, 'app'))
    )
    renderTasksFn.mockResolvedValue({
      items: [
        renderTask('team-a', 'app', 'JobSucceeded'),
        renderTask('shared', 'app', 'JobFailed'),
      ],
    })

    renderList(
      target([
        { namespace: 'shared', name: 'rb-2', uniqueName: 'app-shared' },
        { namespace: 'team-a', name: 'rb-1', uniqueName: 'app' },
      ])
    )

    expect(
      await within(await screen.findByTestId('release-row-team-a/rb-1')).findByText('Succeeded')
    ).toBeInTheDocument()
    expect(
      within(screen.getByTestId('release-row-shared/rb-2')).getByText('Render failed')
    ).toBeInTheDocument()
  })

  it('shows why a binding is unresolved', async () => {
    const unresolved = binding('team-a', 'rb-1', 'gone')
    unresolved.status = {
      conditions: [
        {
          type: 'Deployed',
          status: 'Unknown',
          reason: 'Unresolved',
          message: 'Release team-a/gone not found',
          lastTransitionTime: NOW,
        },
      ],
    }
    bindingFn.mockResolvedValue(unresolved)
    renderTasksFn.mockResolvedValue({ items: [] })

    renderList(
      target([{ namespace: 'team-a', name: 'rb-1', message: 'Release team-a/gone not found' }])
    )

    const row = await screen.findByTestId('release-row-team-a/rb-1')
    expect(await within(row).findByText('Unresolved')).toBeInTheDocument()
    expect(within(row).getByText('Release team-a/gone not found')).toBeInTheDocument()
  })
})
