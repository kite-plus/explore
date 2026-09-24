// shadcn-admin's theme context, wired to the site's own theme: the choice is
// stored under localStorage "theme" and applied as data-theme on <html>,
// exactly as src/scripts/theme.js does on the reader pages.
import { createContext, useContext, useEffect, useState } from 'react'
import { readPreference, writePreference } from '@/admin/lib/storage'

type Theme = 'dark' | 'light' | 'system'
type ResolvedTheme = Exclude<Theme, 'system'>

type ThemeProviderState = {
  resolvedTheme: ResolvedTheme
  theme: Theme
  setTheme: (theme: Theme) => void
}

const ThemeContext = createContext<ThemeProviderState | null>(null)

const STORAGE_KEY = 'theme'
const DARK_QUERY = '(prefers-color-scheme: dark)'

function readTheme(): Theme {
  const saved = readPreference(STORAGE_KEY)
  return saved === 'light' || saved === 'dark' ? saved : 'system'
}

function applyTheme(theme: Theme) {
  const root = document.documentElement
  if (theme === 'system') delete root.dataset.theme
  else root.dataset.theme = theme
  root.dataset.themeMode = theme === 'system' ? 'auto' : theme
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, _setTheme] = useState<Theme>(readTheme)
  const [systemDark, setSystemDark] = useState(
    () => window.matchMedia(DARK_QUERY).matches
  )

  useEffect(() => {
    const media = window.matchMedia(DARK_QUERY)
    const onChange = () => setSystemDark(media.matches)
    media.addEventListener('change', onChange)
    return () => media.removeEventListener('change', onChange)
  }, [])

  // theme.js applies choices made in other tabs; this keeps the state in step.
  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.key === STORAGE_KEY || event.key === null) _setTheme(readTheme())
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  const setTheme = (next: Theme) => {
    writePreference(STORAGE_KEY, next === 'system' ? undefined : next)
    applyTheme(next)
    _setTheme(next)
  }

  const resolvedTheme: ResolvedTheme =
    theme === 'system' ? (systemDark ? 'dark' : 'light') : theme

  return (
    <ThemeContext value={{ theme, resolvedTheme, setTheme }}>
      {children}
    </ThemeContext>
  )
}

export const useTheme = () => {
  const context = useContext(ThemeContext)
  if (!context) throw new Error('useTheme must be used within a ThemeProvider')
  return context
}
