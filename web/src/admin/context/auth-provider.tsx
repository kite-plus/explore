import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from 'react'
import { ACCOUNT_ADMIN } from '@/lib/admin-api'
import { currentReader, readerRequest } from '@/lib/reader-api'

type AdminUser = { name: string; email: string }

type AuthContextType = {
  /** A maintainer token, ACCOUNT_ADMIN for a signed-in account, or null. */
  token: string | null
  checking: boolean
  user: AdminUser | null
  error: string | null
  login: (token: string, remember: boolean) => void
  logout: () => void
  /** Called by admin requests that get a 401. */
  onUnauthorized: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

const STORAGE_KEY = 'explore_admin_token'
const TOKEN_USER: AdminUser = { name: '维护者', email: '令牌登录' }

function readToken() {
  try {
    return sessionStorage.getItem(STORAGE_KEY) ?? localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

function clearToken() {
  try {
    sessionStorage.removeItem(STORAGE_KEY)
    localStorage.removeItem(STORAGE_KEY)
  } catch {
    // Nothing was saved when storage is blocked.
  }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string | null>(null)
  const [checking, setChecking] = useState(true)
  const [user, setUser] = useState<AdminUser | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const stored = readToken()
    if (stored) {
      setToken(stored)
      setChecking(false)
      return
    }
    // An admin account signs in with the reader session cookie.
    void fetch('/api/v1/admin/session', { credentials: 'same-origin' })
      .then((response) => {
        if (response.ok) setToken(ACCOUNT_ADMIN)
      })
      .catch(() => {})
      .finally(() => setChecking(false))
  }, [])

  useEffect(() => {
    if (token === null) {
      setUser(null)
      return
    }
    if (token !== ACCOUNT_ADMIN) {
      setUser(TOKEN_USER)
      return
    }
    void currentReader().then(
      (reader) => reader && setUser({ name: reader.display_name, email: reader.email })
    )
  }, [token])

  const login = useCallback((next: string, remember: boolean) => {
    clearToken()
    if (next !== ACCOUNT_ADMIN) {
      try {
        ;(remember ? localStorage : sessionStorage).setItem(STORAGE_KEY, next)
      } catch {
        // The token then lasts for this page only.
      }
    }
    setToken(next)
    setError(null)
  }, [])

  const logout = useCallback(() => {
    if (token === ACCOUNT_ADMIN) {
      void currentReader().then(
        (reader) =>
          reader &&
          readerRequest('auth/logout', { method: 'POST' }, reader.csrf_token)
      )
    }
    clearToken()
    setToken(null)
    setError(null)
  }, [token])

  const onUnauthorized = useCallback(() => {
    clearToken()
    setToken(null)
    setError('登录已失效，请重新登录。')
  }, [])

  return (
    <AuthContext
      value={{ token, checking, user, error, login, logout, onUnauthorized }}
    >
      {children}
    </AuthContext>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw new Error('useAuth must be used within AuthProvider')
  return context
}
