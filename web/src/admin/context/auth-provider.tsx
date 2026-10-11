import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from 'react'
import { currentReader, readerRequest, type AuthOptions, type ReaderUser } from '@/lib/reader-api'

type AdminUser = { name: string; email: string }

/** A fresh install shows the setup wizard until its first admin exists. */
export type AuthStatus = 'checking' | 'setup' | 'signed-out' | 'signed-in'

type AuthContextType = {
  status: AuthStatus
  user: AdminUser | null
  /**
   * The server signs in with Kite Plus alone: no passwords to sign in with,
   * change or reset, and no setup by password (docs/design/accounts.md
   * section 1).
   */
  kitePlusOnly: boolean
  /** Signed in, but to an account without admin access; Kite Plus only. */
  reader: ReaderUser | null
  error: string | null
  /** Takes the account after sign-in, setup or a change to its profile. */
  signedIn: (account: ReaderUser) => void
  logout: () => void
  /** Called by admin requests that get a 401. */
  onUnauthorized: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

async function signInOptions(): Promise<AuthOptions | null> {
  try {
    const response = await fetch('/api/v1/auth/options')
    return response.ok ? ((await response.json()) as AuthOptions) : null
  } catch {
    return null
  }
}

async function initialStatus(): Promise<{ status: AuthStatus; account: ReaderUser | null; kitePlusOnly: boolean; reader: ReaderUser | null }> {
  const [session, options] = await Promise.all([fetch('/api/v1/admin/session', { credentials: 'same-origin' }), signInOptions()])
  const kitePlusOnly = Boolean(options?.oidc) && options?.password === false
  const account = await currentReader()
  if (session.ok && account) return { status: 'signed-in', account, kitePlusOnly, reader: null }
  const setup = await fetch('/api/v1/setup')
  const state: { required?: boolean } | null = setup.ok ? await setup.json() : null
  return { status: state?.required ? 'setup' : 'signed-out', account: null, kitePlusOnly, reader: account }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>('checking')
  const [user, setUser] = useState<AdminUser | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [kitePlusOnly, setKitePlusOnly] = useState(false)
  const [reader, setReader] = useState<ReaderUser | null>(null)

  useEffect(() => {
    initialStatus()
      .then(({ status, account, kitePlusOnly, reader }) => {
        if (account) setUser({ name: account.display_name, email: account.email })
        setKitePlusOnly(kitePlusOnly)
        setReader(reader)
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
    setReader(null)
    setStatus('signed-out')
    setError(null)
  }, [])

  const onUnauthorized = useCallback(() => {
    setUser(null)
    setStatus('signed-out')
    setError('登录已失效，请重新登录。')
  }, [])

  return (
    <AuthContext value={{ status, user, error, kitePlusOnly, reader, signedIn, logout, onUnauthorized }}>
      {children}
    </AuthContext>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) throw new Error('useAuth must be used within AuthProvider')
  return context
}
