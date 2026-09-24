import React, { useState } from 'react'
import useDialogState from '@/admin/hooks/use-dialog-state'
import type { AdminSubmission } from '@/lib/admin-types'

type SubmissionsDialogType = 'detail' | 'approve' | 'reject'

type SubmissionsContextType = {
  open: SubmissionsDialogType | null
  setOpen: (str: SubmissionsDialogType | null) => void
  currentRow: AdminSubmission | null
  setCurrentRow: React.Dispatch<React.SetStateAction<AdminSubmission | null>>
}

const SubmissionsContext = React.createContext<SubmissionsContextType | null>(null)

export function SubmissionsProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useDialogState<SubmissionsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<AdminSubmission | null>(null)
  return (
    <SubmissionsContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </SubmissionsContext>
  )
}

export const useSubmissions = () => {
  const context = React.useContext(SubmissionsContext)
  if (!context) throw new Error('useSubmissions has to be used within <SubmissionsContext>')
  return context
}
