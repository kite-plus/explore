// A small client-side router in place of shadcn-admin's TanStack Router:
// Astro serves every /admin path with the same page, and this switches
// pages with the History API. Search params round-trip like TanStack
// Router's: values are JSON when they parse as JSON, plain strings otherwise.
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type AnchorHTMLAttributes,
} from 'react'

export type SearchRecord = Record<string, unknown>

type Location = {
  pathname: string
  search: SearchRecord
  href: string
}

export type NavigateOptions = {
  to?: string
  search?: true | SearchRecord | ((prev: SearchRecord) => SearchRecord)
  replace?: boolean
}

type RouterContextType = {
  location: Location
  navigate: (options: NavigateOptions | string) => void
}

const RouterContext = createContext<RouterContextType | null>(null)

function parseValue(raw: string): unknown {
  try {
    return JSON.parse(raw)
  } catch {
    return raw
  }
}

function formatValue(value: unknown): string {
  if (typeof value !== 'string') return JSON.stringify(value)
  // Quote strings that would otherwise come back as another type.
  try {
    JSON.parse(value)
    return JSON.stringify(value)
  } catch {
    return value
  }
}

export function parseSearch(query: string): SearchRecord {
  const search: SearchRecord = {}
  for (const [key, value] of new URLSearchParams(query)) {
    search[key] = parseValue(value)
  }
  return search
}

export function stringifySearch(search: SearchRecord): string {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(search)) {
    if (value === undefined || value === null || value === '') continue
    params.set(key, formatValue(value))
  }
  const query = params.toString()
  return query ? `?${query}` : ''
}

export function normalizePath(pathname: string) {
  return pathname.length > 1 ? pathname.replace(/\/+$/, '') : pathname
}

function readLocation(): Location {
  const pathname = normalizePath(window.location.pathname)
  return {
    pathname,
    search: parseSearch(window.location.search),
    href: pathname + window.location.search,
  }
}

export function RouterProvider({ children }: { children: React.ReactNode }) {
  const [location, setLocation] = useState(readLocation)

  useEffect(() => {
    const onPopState = () => setLocation(readLocation())
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])

  const navigate = useCallback((options: NavigateOptions | string) => {
    const opts = typeof options === 'string' ? { to: options } : options
    const current = readLocation()
    const [path, query] = (opts.to ?? current.pathname).split('?')
    let search: SearchRecord =
      query !== undefined ? parseSearch(query) : opts.to ? {} : current.search
    if (opts.search === true) search = current.search
    else if (typeof opts.search === 'function') search = opts.search(current.search)
    else if (opts.search) search = opts.search

    const pathname = normalizePath(path)
    const href = pathname + stringifySearch(search)
    if (href === current.href) return
    window.history[opts.replace ? 'replaceState' : 'pushState'](null, '', href)
    setLocation(readLocation())
    if (pathname !== current.pathname) window.scrollTo(0, 0)
  }, [])

  return (
    <RouterContext value={{ location, navigate }}>{children}</RouterContext>
  )
}

function useRouter() {
  const context = useContext(RouterContext)
  if (!context) throw new Error('useRouter must be used within RouterProvider')
  return context
}

export const useLocation = () => useRouter().location
export const useNavigate = () => useRouter().navigate
export const useSearch = () => useRouter().location.search

type LinkProps = Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> & {
  to: string
}

/** An anchor that switches pages in place; modified clicks still open a tab. */
export function Link({ to, onClick, target, ...props }: LinkProps) {
  const { navigate } = useRouter()
  return (
    <a
      href={to}
      target={target}
      onClick={(event) => {
        onClick?.(event)
        if (
          event.defaultPrevented ||
          event.button !== 0 ||
          event.metaKey ||
          event.ctrlKey ||
          event.shiftKey ||
          event.altKey ||
          (target && target !== '_self')
        )
          return
        event.preventDefault()
        navigate(to)
      }}
      {...props}
    />
  )
}
