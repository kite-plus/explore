import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { toastError, useAdminRequest } from '@/admin/lib/api'
import type { AdminEntryRow } from '@/lib/admin-types'

/** Hides or restores entries; the reason is required when hiding. */
export function useSetHidden() {
  const request = useAdminRequest()
  const queryClient = useQueryClient()
  return async (entries: AdminEntryRow[], hidden: boolean, reason = '') => {
    const results = await Promise.allSettled(
      entries.map((entry) =>
        request(`/entries/${entry.id}`, {
          method: 'PATCH',
          body: JSON.stringify({ hidden, reason }),
        })
      )
    )
    await queryClient.invalidateQueries({ queryKey: ['admin'] })
    const failed = results.filter((result) => result.status === 'rejected') as PromiseRejectedResult[]
    const done = entries.length - failed.length
    const verb = hidden ? '隐藏' : '恢复展示'
    if (done > 0)
      toast.success(
        entries.length === 1 ? `已${verb}：${entries[0].title}` : `已${verb} ${done} 篇文章`
      )
    if (failed.length) toastError(failed[0].reason, `${failed.length} 篇文章${verb}失败`)
    return failed.length === 0
  }
}
