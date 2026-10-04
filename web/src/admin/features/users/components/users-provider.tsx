import React, { useRef, useState } from 'react'
import { useAuth } from '@/admin/context/auth-provider'
import { useNavigate, useSearch } from '@/admin/router'
import type { AdminUserRow } from '@/lib/admin-types'

export type UsersDialogType = 'disable' | 'restore' | 'admin' | 'rename' | 'sign-out' | 'delete' | 'release'

type UsersContextType = {
  open: UsersDialogType | null
  /** The accounts the open dialog acts on: one row, or a bulk selection. */
  targets: AdminUserRow[]
  /** The blog the release dialog gives up. */
  host: string
  act: (dialog: UsersDialogType, targets: AdminUserRow[], options?: { host?: string; onDone?: () => void }) => void
  close: () => void
  /** Called by a dialog once its change succeeded, as when a bulk action clears the selection. */
  done: () => void
  /** The account whose detail drawer is open; kept in the address so it can be linked. */
  detailId: string | null
  showDetail: (id: string | null) => void
}

const UsersContext = React.createContext<UsersContextType | null>(null)

export function UsersProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState<UsersDialogType | null>(null)
  // Targets outlive the dialog so its content stays put while it animates out.
  const [targets, setTargets] = useState<AdminUserRow[]>([])
  const [host, setHost] = useState('')
  const onDone = useRef<(() => void) | undefined>(undefined)
  const search = useSearch()
  const navigate = useNavigate()
  const detailId = typeof search.user === 'string' ? search.user : null

  const value: UsersContextType = {
    open,
    targets,
    host,
    act: (dialog, rows, options = {}) => {
      setTargets(rows)
      setHost(options.host ?? '')
      onDone.current = options.onDone
      setOpen(dialog)
    },
    close: () => setOpen(null),
    done: () => {
      setOpen(null)
      onDone.current?.()
    },
    detailId,
    showDetail: (id) => navigate({ search: (prev) => ({ ...prev, user: id ?? undefined }) }),
  }
  return <UsersContext value={value}>{children}</UsersContext>
}

export const useUsers = () => {
  const context = React.useContext(UsersContext)
  if (!context) throw new Error('useUsers has to be used within <UsersContext>')
  return context
}

/** The API refuses most changes to one's own account. */
export function useIsSelf() {
  const { user: me } = useAuth()
  return (user: AdminUserRow) => me?.email === user.email
}
