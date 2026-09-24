import React, { useState } from 'react'
import useDialogState from '@/admin/hooks/use-dialog-state'
import type { Takedown } from '@/lib/admin-types'

type TakedownsDialogType = 'create' | 'review'

type TakedownsContextType = {
  open: TakedownsDialogType | null
  setOpen: (str: TakedownsDialogType | null) => void
  currentRow: Takedown | null
  setCurrentRow: React.Dispatch<React.SetStateAction<Takedown | null>>
}

const TakedownsContext = React.createContext<TakedownsContextType | null>(null)

export function TakedownsProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useDialogState<TakedownsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<Takedown | null>(null)
  return (
    <TakedownsContext value={{ open, setOpen, currentRow, setCurrentRow }}>{children}</TakedownsContext>
  )
}

export const useTakedowns = () => {
  const context = React.useContext(TakedownsContext)
  if (!context) throw new Error('useTakedowns has to be used within <TakedownsContext>')
  return context
}

export function targetName(item: Takedown) {
  return item.target_type === 'blog'
    ? item.blog_host
    : item.entry_title || item.entry_identity || '（文章已不在缓存中）'
}
