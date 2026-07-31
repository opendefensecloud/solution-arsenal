// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { ConditionsTable } from './conditions-table'

describe('ConditionsTable', () => {
  it('renders "No conditions" when empty', () => {
    render(<ConditionsTable conditions={[]} />)
    expect(screen.getByText('No conditions')).toBeInTheDocument()
  })

  it('renders a row per condition with the Last Transition column', () => {
    render(
      <ConditionsTable
        conditions={[
          {
            type: 'Ready',
            status: 'True',
            reason: 'AllGood',
            message: 'everything works',
            lastTransitionTime: '2026-07-28T10:00:00Z',
          },
        ]}
      />
    )
    expect(screen.getByText('Ready')).toBeInTheDocument()
    expect(screen.getByText('AllGood')).toBeInTheDocument()
    expect(screen.getByText('everything works')).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Last Transition' })).toBeInTheDocument()
  })
})
