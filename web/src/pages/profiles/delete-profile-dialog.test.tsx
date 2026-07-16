// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

const { mutate } = vi.hoisted(() => ({ mutate: vi.fn() }))
vi.mock('@/api/mutations', () => ({ useProfileDelete: () => ({ mutate, isPending: false }) }))
vi.mock('@/components/ui/toast', () => ({ useToast: () => ({ toast: vi.fn() }) }))
vi.mock('@tanstack/react-router', () => ({ useNavigate: () => vi.fn() }))

import { DeleteProfileDialog } from './delete-profile-dialog'

describe('DeleteProfileDialog', () => {
  it('warns about the ReleaseBinding cascade and deletes on confirm', () => {
    render(<DeleteProfileDialog open onOpenChange={() => {}} namespace="default" name="p1" />)
    expect(screen.getByText(/releasebinding/i)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /^delete$/i }))
    expect(mutate).toHaveBeenCalledWith('p1', expect.any(Object))
  })
})
