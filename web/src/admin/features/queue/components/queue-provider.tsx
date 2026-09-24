import React, { useState } from 'react'
import type { FetchQueueItem } from '@/lib/admin-types'

type QueueContextType = {
  attemptsFor: FetchQueueItem | null
  setAttemptsFor: React.Dispatch<React.SetStateAction<FetchQueueItem | null>>
}

const QueueContext = React.createContext<QueueContextType | null>(null)

export function QueueProvider({ children }: { children: React.ReactNode }) {
  const [attemptsFor, setAttemptsFor] = useState<FetchQueueItem | null>(null)
  return <QueueContext value={{ attemptsFor, setAttemptsFor }}>{children}</QueueContext>
}

export const useQueue = () => {
  const context = React.useContext(QueueContext)
  if (!context) throw new Error('useQueue has to be used within <QueueContext>')
  return context
}
