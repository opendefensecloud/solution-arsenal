// Copyright 2026 BWI GmbH and Solution Arsenal contributors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import { CheckCircle2, XCircle, X } from 'lucide-react'
import { cn } from '@/lib/utils'

type Variant = 'success' | 'error'
type ToastOptions = { autoClose?: boolean }
type Toast = { id: number; msg: string; variant: Variant }

const ToastContext = createContext<{
  toast: (msg: string, variant?: Variant, opts?: ToastOptions) => void
} | null>(null)

let nextId = 0
const AUTO_CLOSE_MS = 4000

const variantStyles: Record<Variant, { box: string; icon: typeof CheckCircle2 }> = {
  success: {
    box: 'border-emerald-600/20 bg-emerald-50 text-emerald-700 dark:border-emerald-500/20 dark:bg-emerald-500/10 dark:text-emerald-400',
    icon: CheckCircle2,
  },
  error: {
    box: 'border-red-600/20 bg-red-50 text-red-700 dark:border-red-500/20 dark:bg-red-500/10 dark:text-red-400',
    icon: XCircle,
  },
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])

  const dismiss = useCallback((id: number) => {
    setToasts((t) => t.filter((x) => x.id !== id))
  }, [])

  const toast = useCallback(
    (msg: string, variant: Variant = 'success', opts?: ToastOptions) => {
      const id = nextId++
      setToasts((t) => [...t, { id, msg, variant }])
      // Errors stay until dismissed; everything else auto-closes.
      const autoClose = opts?.autoClose ?? variant !== 'error'
      if (autoClose) setTimeout(() => dismiss(id), AUTO_CLOSE_MS)
    },
    [dismiss]
  )

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div className="fixed bottom-4 right-4 z-[60] flex flex-col gap-2">
        {toasts.map((t) => {
          const { box, icon: Icon } = variantStyles[t.variant]
          return (
            <div
              key={t.id}
              role={t.variant === 'error' ? 'alert' : 'status'}
              className={cn(
                'flex items-start gap-2 rounded-md border px-4 py-2 text-sm shadow-md',
                box
              )}
            >
              <Icon className="mt-0.5 h-4 w-4 shrink-0" />
              <span className="flex-1">{t.msg}</span>
              <button
                type="button"
                aria-label="Dismiss"
                onClick={() => dismiss(t.id)}
                className="mt-0.5 shrink-0 opacity-60 hover:opacity-100"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          )
        })}
      </div>
    </ToastContext.Provider>
  )
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
