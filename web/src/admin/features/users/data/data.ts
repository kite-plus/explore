import { Shield, User, UserCheck, UserX } from 'lucide-react'
import type { AdminUserRow } from '@/lib/admin-types'

export const roles = [
  { label: '管理员', value: 'admin', icon: Shield },
  { label: '读者', value: 'reader', icon: User },
] as const

export const userStatuses = [
  { label: '正常', value: 'active', icon: UserCheck, className: 'text-success' },
  { label: '已停用', value: 'disabled', icon: UserX, className: 'text-destructive' },
] as const

export function userStatus(user: AdminUserRow) {
  return userStatuses[user.disabled_at ? 1 : 0]
}

export function userRole(user: AdminUserRow) {
  return roles[user.is_admin ? 0 : 1]
}

/** Columns the API can order by, keyed by column id. */
export const sortKeys: Record<string, string> = {
  number: 'number',
  created_at: 'created',
  last_seen_at: 'seen',
  subscription_count: 'subscriptions',
  owned_blog_count: 'blogs',
}

export const reportStatuses = {
  pending: { label: '待处理', variant: 'outline' },
  approved: { label: '已下架', variant: 'secondary' },
  rejected: { label: '已驳回', variant: 'secondary' },
} as const

export function adminCopy(grant: boolean) {
  return grant
    ? { title: '授予后台权限', desc: '这个账号将可以进入后台，处理投稿、内容和用户。', action: '授予权限', destructive: false }
    : { title: '取消后台权限', desc: '取消后这个账号的会话会被撤销，需要重新登录。', action: '取消权限', destructive: true }
}
