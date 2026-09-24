import React, { useState } from 'react'
import useDialogState from '@/admin/hooks/use-dialog-state'
import type { AdminEntryRow } from '@/lib/admin-types'

type EntriesDialogType = 'hide'

type EntriesContextType = {
  open: EntriesDialogType | null
  setOpen: (str: EntriesDialogType | null) => void
  /** The entries the hide dialog acts on: one row, or a bulk selection. */
  targets: AdminEntryRow[]
  setTargets: React.Dispatch<React.SetStateAction<AdminEntryRow[]>>
}

const EntriesContext = React.createContext<EntriesContextType | null>(null)

export function EntriesProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useDialogState<EntriesDialogType>(null)
  const [targets, setTargets] = useState<AdminEntryRow[]>([])
  return <EntriesContext value={{ open, setOpen, targets, setTargets }}>{children}</EntriesContext>
}

export const useEntries = () => {
  const context = React.useContext(EntriesContext)
  if (!context) throw new Error('useEntries has to be used within <EntriesContext>')
  return context
}
