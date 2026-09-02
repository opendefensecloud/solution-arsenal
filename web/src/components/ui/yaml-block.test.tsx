// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { YamlBlock } from './yaml-block'

describe('YamlBlock', () => {
  it('renders nothing for undefined', () => {
    const { container } = render(<YamlBlock value={undefined} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders nothing for an empty object', () => {
    const { container } = render(<YamlBlock value={{}} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders YAML for an object', () => {
    render(<YamlBlock value={{ foo: 'bar' }} />)
    expect(screen.getByText(/foo: bar/)).toBeInTheDocument()
  })
})
