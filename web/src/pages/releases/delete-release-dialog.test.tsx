// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

const { mutate } = vi.hoisted(() => ({ mutate: vi.fn() }))
vi.mock('@/api/mutations', () => ({ useReleaseDelete: () => ({ mutate, isPending: false }) }))
vi.mock('@/components/ui/toast', () => ({ useToast: () => ({ toast: vi.fn() }) }))
vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }))

import { DeleteReleaseDialog } from './delete-release-dialog'

describe('DeleteReleaseDialog', () => {
  it('explains blocking references and deletes on confirm', () => {
    render(<DeleteReleaseDialog open onOpenChange={() => {}} namespace="default" name="r1" />)
    expect(screen.getByText(/stays in deletion/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))
    expect(mutate).toHaveBeenCalledWith('r1', expect.any(Object))
  })
})
