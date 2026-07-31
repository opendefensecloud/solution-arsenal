// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { render, screen, cleanup } from '@testing-library/react'
import { describe, it, expect, vi, afterEach } from 'vitest'
import { Server } from 'lucide-react'
import { DetailHeader } from './detail-header'
import { StatGrid } from './stat-grid'
import { DetailSection } from './detail-section'

describe('detail primitives', () => {
  afterEach(() => {
    cleanup()
  })
  it('DetailHeader shows title, namespace and calls onBack', () => {
    const onBack = vi.fn()
    render(
      <DetailHeader
        icon={Server}
        title="my-target"
        namespace="team-a"
        backLabel="Back to Targets"
        onBack={onBack}
      />
    )
    expect(screen.getByRole('heading', { level: 1, name: 'my-target' })).toBeInTheDocument()
    expect(screen.getByText('team-a')).toBeInTheDocument()
    screen.getByRole('button', { name: 'Back to Targets' }).click()
    expect(onBack).toHaveBeenCalledOnce()
  })

  it('StatGrid renders a card per stat', () => {
    render(<StatGrid stats={[{ label: 'Namespace', value: 'team-a' }]} />)
    expect(screen.getByText('Namespace')).toBeInTheDocument()
    expect(screen.getByText('team-a')).toBeInTheDocument()
  })

  it('DetailSection appends the count to the heading', () => {
    render(
      <DetailSection title="Bound Releases" count={2}>
        <span>child</span>
      </DetailSection>
    )
    expect(screen.getByRole('heading', { name: 'Bound Releases (2)' })).toBeInTheDocument()
    expect(screen.getByText('child')).toBeInTheDocument()
  })
})
