import { BookOpen, CircleCheck, CircleX, Clock, FileText } from 'lucide-react'

export const takedownStatuses = [
  { label: '待处理', value: 'pending' as const, icon: Clock, className: 'text-warning' },
  { label: '已下架', value: 'approved' as const, icon: CircleCheck, className: 'text-success' },
  { label: '已驳回', value: 'rejected' as const, icon: CircleX, className: 'text-destructive' },
]

export const targetTypes = [
  { label: '博客', value: 'blog' as const, icon: BookOpen },
  { label: '文章', value: 'entry' as const, icon: FileText },
]

export const statusOrder = { pending: 0, approved: 1, rejected: 2 } as const
