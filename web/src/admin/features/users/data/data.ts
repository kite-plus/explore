import { Shield, User } from 'lucide-react'

export const roles = [
  { label: '管理员', value: 'admin', icon: Shield },
  { label: '读者', value: 'reader', icon: User },
] as const

// Status badge colors, as in shadcn-admin's users table.
export const statusStyles = {
  active: 'bg-teal-100/30 text-teal-900 dark:text-teal-200 border-teal-200',
  disabled:
    'bg-destructive/10 dark:bg-destructive/50 text-destructive dark:text-primary border-destructive/10',
} as const

export type UserChange = { field: 'disabled' | 'is_admin'; value: boolean }

export function changeCopy(change: UserChange) {
  if (change.field === 'disabled')
    return change.value
      ? { title: '停用账号', desc: '停用后这个账号不能登录，现有会话立即失效。订阅和认领记录会保留。', action: '停用', destructive: true }
      : { title: '恢复账号', desc: '恢复后这个账号可以重新登录。', action: '恢复', destructive: false }
  return change.value
    ? { title: '授予后台权限', desc: '这个账号将可以进入后台，处理投稿、内容和用户。', action: '授予权限', destructive: false }
    : { title: '取消后台权限', desc: '取消后这个账号的会话会被撤销，需要重新登录。', action: '取消权限', destructive: true }
}
