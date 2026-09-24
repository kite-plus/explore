import React, { useState } from 'react'
import type { AdminUserRow } from '@/lib/admin-types'
import type { UserChange } from '../data/data'

type UsersContextType = {
  pending: { user: AdminUserRow; change: UserChange } | null
  setPending: React.Dispatch<React.SetStateAction<{ user: AdminUserRow; change: UserChange } | null>>
}

const UsersContext = React.createContext<UsersContextType | null>(null)

export function UsersProvider({ children }: { children: React.ReactNode }) {
  const [pending, setPending] = useState<UsersContextType['pending']>(null)
  return <UsersContext value={{ pending, setPending }}>{children}</UsersContext>
}

export const useUsers = () => {
  const context = React.useContext(UsersContext)
  if (!context) throw new Error('useUsers has to be used within <UsersContext>')
  return context
}
