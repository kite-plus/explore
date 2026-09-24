import React, { useState } from 'react'
import useDialogState from '@/admin/hooks/use-dialog-state'
import type { AdminBlog } from '@/lib/admin-types'

type BlogsDialogType = 'create' | 'detail' | 'edit' | 'delete'

type BlogsContextType = {
  open: BlogsDialogType | null
  setOpen: (str: BlogsDialogType | null) => void
  currentRow: AdminBlog | null
  setCurrentRow: React.Dispatch<React.SetStateAction<AdminBlog | null>>
}

const BlogsContext = React.createContext<BlogsContextType | null>(null)

export function BlogsProvider({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useDialogState<BlogsDialogType>(null)
  const [currentRow, setCurrentRow] = useState<AdminBlog | null>(null)

  return (
    <BlogsContext value={{ open, setOpen, currentRow, setCurrentRow }}>
      {children}
    </BlogsContext>
  )
}

export const useBlogs = () => {
  const context = React.useContext(BlogsContext)
  if (!context) throw new Error('useBlogs has to be used within <BlogsContext>')
  return context
}
