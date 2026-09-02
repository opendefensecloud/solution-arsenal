// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import type { Condition } from '@/api/types'
import { StatusDot } from './status-dot'

function statusColor(s: Condition['status']) {
  return s === 'True' ? 'success' : s === 'False' ? 'danger' : 'muted'
}

const HEADERS = ['Type', 'Status', 'Reason', 'Message', 'Last Transition'] as const

export function ConditionsTable({ conditions }: { conditions?: Condition[] }) {
  if (!conditions?.length) return <p className="text-sm text-muted-foreground">No conditions</p>
  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-border bg-muted/30">
            {HEADERS.map((h) => (
              <th key={h} className="px-3 py-2 text-left font-medium text-muted-foreground">
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {conditions.map((c) => (
            <tr key={c.type} className="border-b border-border last:border-b-0">
              <td className="px-3 py-2 font-mono font-medium text-foreground">{c.type}</td>
              <td className="px-3 py-2">
                <StatusDot color={statusColor(c.status)} label={c.status} />
              </td>
              <td className="px-3 py-2 font-mono text-muted-foreground">{c.reason || '—'}</td>
              <td className="px-3 py-2 text-muted-foreground">{c.message || '—'}</td>
              <td className="px-3 py-2 whitespace-nowrap text-muted-foreground">
                {c.lastTransitionTime ? new Date(c.lastTransitionTime).toLocaleString() : '—'}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
