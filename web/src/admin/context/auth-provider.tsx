import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from 'react'
import { currentReader, readerRequest, type ReaderUser } from '@/lib/reader-api'

type AdminUser = { name: string; email: string }

/** A fresh install shows the setup wizard until its first admin exists. */
export type AuthStatus = 'checking' | 'setup' | 'signed-out' | 'signed-in'

type AuthContextType = {
  status: AuthStatus
  user: AdminUser | null
  error: string | null
  /** Takes the account that just signed in or finished setup. */
  signedIn: (account: ReaderUser) => void
  logout: () => void
  /** Called by admin requests that get a 401. */
  onUnauthorized: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

async function initialStatus(): Promise<{ status: AuthStatus; account: ReaderUser | null }> {
  const session = await fetch('/api/v1/admin/session', { credentials: 'same-origin' })
  if (session.ok) {
    const account = await currentReader()
    if (account) return { status: 'signed-in', account }
  }
  const setup = await fetch('/api/v1/setup')
  const state: { required?: boolean } | null = setup.ok ? await setup.json() : null
  return { status: state?.required ? 'setup' : 'signed-out', account: null }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>('checking')
  const [user, setUser] = useState<AdminUser | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    initialStatus()
      .then(({ status, account }) => {
        if (account) setUser({ name: account.display_name, email: account.email })
        setStatus(status)
      })
      .catch(() => setStatus('signed-out'))
  }, [])

  const signedIn = useCallback((account: ReaderUser) => {
    setUser({ name: account.display_name, email: account.email })
    setStatus('signed-in')
    setError(null)
  }, [])

  const logout = useCallback(() => {
    void currentReader().then(
      (account) => account && readerRequest('auth/logout', { method: 'POST' }, account.csrf_token)
    )
    setUser(null)
    setStatus('signed-out')
    setError(null)
  }, [])

  const onUnauthorized = useCallback(() => {
    setUser(null)
    setStatus('signed-out')
    setError('登录已失效，请重新登录。')
  }, [])

  return (
    <AuthContext value={{ status, user, error, signedIn, logout, onUnauthorized }}>
      {children}
    </AuthContext>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw new Error('useAuth must be used within AuthProvider')
  return context
}
