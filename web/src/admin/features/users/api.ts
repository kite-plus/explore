import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { toastError, useAdminQuery, useAdminRequest } from '@/admin/lib/api'
import type { AdminUserDetail, AdminUserRow } from '@/lib/admin-types'

export function useUserDetail(id: string | null) {
  return useAdminQuery<AdminUserDetail>(`/users/${encodeURIComponent(id ?? '')}`, { enabled: Boolean(id) })
}

type Change = { path: string; method: string; body?: unknown }

/**
 * Account changes, one request per account. Each reports its outcome in a
 * toast and resolves to whether every request succeeded.
 */
export function useUserActions() {
  const request = useAdminRequest()
  const queryClient = useQueryClient()

  async function run(users: AdminUserRow[], change: (path: string) => Change, done: string) {
    const results = await Promise.allSettled(
      users.map((user) => {
        const { path, method, body } = change(`/users/${encodeURIComponent(user.id)}`)
        return request(path, { method, body: body === undefined ? undefined : JSON.stringify(body) })
      })
    )
    await queryClient.invalidateQueries({ queryKey: ['admin'] })
    const failed = results.filter((result) => result.status === 'rejected') as PromiseRejectedResult[]
    const succeeded = users.length - failed.length
    if (succeeded > 0)
      toast.success(users.length === 1 ? `${users[0].email}：${done}` : `${done} ${succeeded} 个账号`)
    if (failed.length) toastError(failed[0].reason, `${failed.length} 个账号操作失败`)
    return failed.length === 0
  }

  return {
    setDisabled: (users: AdminUserRow[], disabled: boolean, reason = '') =>
      run(users, (path) => ({ path, method: 'PATCH', body: disabled ? { disabled, reason } : { disabled } }),
        disabled ? '已停用' : '已恢复'),
    setAdmin: (user: AdminUserRow, isAdmin: boolean) =>
      run([user], (path) => ({ path, method: 'PATCH', body: { is_admin: isAdmin } }),
        isAdmin ? '已授予后台权限' : '已取消后台权限'),
    resetPassword: (user: AdminUserRow, password: string) =>
      run([user], (path) => ({ path: `${path}/password`, method: 'PUT', body: { password } }), '密码已重置'),
    signOut: (users: AdminUserRow[]) =>
      run(users, (path) => ({ path: `${path}/sessions`, method: 'DELETE' }), '已强制下线'),
    release: (user: AdminUserRow, host: string) =>
      run([user], (path) => ({ path: `${path}/blogs/${encodeURIComponent(host)}`, method: 'DELETE' }),
        `已解除 ${host} 的认领`),
    remove: (user: AdminUserRow) => run([user], (path) => ({ path, method: 'DELETE' }), '已删除'),
  }
}
