import { useState, useEffect, useCallback } from 'react'

type Theme = 'light' | 'dark' | 'system'

const THEME_STORAGE_KEY = 'solar-theme'

function isTheme(value: string): value is Theme {
  return value === 'light' || value === 'dark' || value === 'system'
}

function loadTheme(): Theme {
  try {
    const stored = localStorage.getItem(THEME_STORAGE_KEY)
    return stored && isTheme(stored) ? stored : 'system'
  } catch {
    // localStorage can throw in private windows / restricted storage modes.
    return 'system'
  }
}

function getSystemTheme(): 'light' | 'dark' {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyTheme(theme: Theme) {
  const resolved = theme === 'system' ? getSystemTheme() : theme
  document.documentElement.classList.toggle('dark', resolved === 'dark')
}

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(loadTheme)

  const setTheme = useCallback((t: Theme) => {
    setThemeState(t)
    try {
      localStorage.setItem(THEME_STORAGE_KEY, t)
    } catch {
      // Ignore storage failures; in-memory state still updates.
    }
    applyTheme(t)
  }, [])

  useEffect(() => {
    applyTheme(theme)

    if (theme === 'system') {
      const mq = window.matchMedia('(prefers-color-scheme: dark)')
      const handler = () => applyTheme('system')
      mq.addEventListener('change', handler)

      return () => mq.removeEventListener('change', handler)
    }
  }, [theme])

  return { theme, setTheme } as const
}
