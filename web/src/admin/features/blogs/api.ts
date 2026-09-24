import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { toastError, useAdminQuery, useAdminRequest } from '@/admin/lib/api'
import { useNavigate } from '@/admin/router'
import type { AdminBlog, FetchQueueSnapshot } from '@/lib/admin-types'

export function useBlogsQuery() {
  return useAdminQuery<{ data: AdminBlog[] }>('/blogs')
}

/** Queues fetches and says whether a worker will pick them up. */
export function useFetchNow() {
  const request = useAdminRequest()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  return async (hosts: string[]) => {
    const results = await Promise.allSettled(
      hosts.map((host) =>
        request(`/blogs/${encodeURIComponent(host)}/fetch`, { method: 'POST' })
      )
    )
    const failed = results.filter((result) => result.status === 'rejected')
    void queryClient.invalidateQueries({ queryKey: ['admin'] })
    if (failed.length === hosts.length) {
      toastError((failed[0] as PromiseRejectedResult).reason, '排队失败，请重试。')
      return
    }
    const queue = await request<FetchQueueSnapshot>('/fetch-queue').catch(() => null)
    const label = hosts.length === 1 ? hosts[0] : `${hosts.length - failed.length} 个博客`
    const action = {
      label: '查看任务',
      onClick: () =>
        navigate({
          to: '/admin/queue',
          search: hosts.length === 1 ? { filter: hosts[0] } : {},
        }),
    }
    if (queue?.crawler_paused)
      toast.warning(`${label} 已排队，但抓取任务已暂停`, {
        description: '在系统设置里恢复后才会执行。',
        action,
      })
    else if (queue && !queue.worker_online)
      toast.warning(`${label} 已排队，但抓取进程离线`, {
        description: '进程启动后才会执行。',
        action,
      })
    else toast.success(`${label} 已排队，等待抓取`, { action })
    if (failed.length) toast.error(`${failed.length} 个博客排队失败`)
  }
}
