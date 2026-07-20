// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import type { Profile } from '@/api/types'

const { mutate } = vi.hoisted(() => ({ mutate: vi.fn() }))
vi.mock('@/api/mutations', () => ({ useProfileUpdate: () => ({ mutate, isPending: false }) }))
vi.mock('@/components/ui/toast', () => ({ useToast: () => ({ toast: vi.fn() }) }))

import { EditProfileDialog } from './edit-profile-dialog'

const profile = {
  metadata: { name: 'p1', namespace: 'default', creationTimestamp: '', uid: 'u' },
  spec: { releaseRef: { name: 'rel' }, targetSelector: { matchLabels: { app: 'web' } } },
} as unknown as Profile

describe('EditProfileDialog', () => {
  it('drops a removed label from the patch so it is replaced away', () => {
    render(<EditProfileDialog open onOpenChange={() => {}} profile={profile} />)
    fireEvent.click(screen.getByRole('button', { name: /remove label/i }))
    fireEvent.click(screen.getByRole('button', { name: /save/i }))
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        spec: expect.objectContaining({ targetSelector: { matchLabels: {} } }),
      }),
      expect.any(Object)
    )
  })
})
