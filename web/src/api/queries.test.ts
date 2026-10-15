// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest'
import { releaseBindingQueries, withFieldSelector } from './queries'

describe('withFieldSelector', () => {
  it('returns the path unchanged without a selector', () => {
    expect(withFieldSelector('/componentversions')).toBe('/componentversions')
  })

  it('encodes OCM names containing slashes and dots', () => {
    const url = withFieldSelector('/namespaces/ns/componentversions', {
      'spec.componentName': 'opendefense.cloud/arc',
    })
    expect(url).toBe(
      '/namespaces/ns/componentversions?fieldSelector=spec.componentName%3Dopendefense.cloud%2Farc'
    )
    expect(new URL(url, 'http://x').searchParams.get('fieldSelector')).toBe(
      'spec.componentName=opendefense.cloud/arc'
    )
  })

  it('joins multiple terms with commas', () => {
    const url = withFieldSelector('/cv', { 'spec.componentName': 'a', 'spec.tag': 'v1' })
    expect(new URL(url, 'http://x').searchParams.get('fieldSelector')).toBe(
      'spec.componentName=a,spec.tag=v1'
    )
  })
})

describe('query keys', () => {
  it('uses different keys for a binding list and a binding detail', () => {
    expect(releaseBindingQueries.list('ns', 'foo').queryKey).not.toEqual(
      releaseBindingQueries.detail('ns', 'foo').queryKey
    )
  })
})
