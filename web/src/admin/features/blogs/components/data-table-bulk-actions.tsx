import { useState } from 'react'
import { type Table } from '@tanstack/react-table'
import { Pause, Play, RefreshCw } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useAdminRequest } from '@/admin/lib/api'
import { Button } from '@/admin/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/admin/components/ui/tooltip'
import { ConfirmDialog } from '@/admin/components/confirm-dialog'
import { DataTableBulkActions as BulkActionsToolbar } from '@/admin/components/data-table'
import type { AdminBlog } from '@/lib/admin-types'
import { useFetchNow } from '../api'

export function DataTableBulkActions({ table }: { table: Table<AdminBlog> }) {
  const request = useAdminRequest()
  const queryClient = useQueryClient()
  const fetchNow = useFetchNow()
  const [pausing, setPausing] = useState(false)
  const [busy, setBusy] = useState(false)
  const selected = table.getFilteredSelectedRowModel().rows.map((row) => row.original)

  async function setStatus(status: AdminBlog['status']) {
    setBusy(true)
    const results = await Promise.allSettled(
      selected.map((blog) =>
        request(`/blogs/${encodeURIComponent(blog.host)}`, {
          method: 'PATCH',
          body: JSON.stringify({ status }),
        })
      )
    )
    const failed = results.filter((result) => result.status === 'rejected').length
    await queryClient.invalidateQueries({ queryKey: ['admin'] })
    setBusy(false)
    setPausing(false)
    table.resetRowSelection()
    const verb = status === 'paused' ? '暂停' : '恢复'
    if (failed) toast.error(`${failed} 个博客${verb}失败`)
    if (failed < selected.length) toast.success(`已${verb} ${selected.length - failed} 个博客`)
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName='博客'>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant='outline'
              size='icon'
              className='size-8'
              aria-label='立即抓取'
              onClick={() => {
                void fetchNow(selected.filter((blog) => blog.status === 'active').map((blog) => blog.host))
                table.resetRowSelection()
              }}
            >
              <RefreshCw />
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            <p>立即抓取（跳过已暂停的博客）</p>
          </TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant='outline'
              size='icon'
              className='size-8'
              aria-label='恢复抓取'
              disabled={busy}
              onClick={() => void setStatus('active')}
            >
              <Play />
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            <p>恢复抓取</p>
          </TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant='destructive'
              size='icon'
              className='size-8'
              aria-label='暂停'
              disabled={busy}
              onClick={() => setPausing(true)}
            >
              <Pause />
            </Button>
          </TooltipTrigger>
          <TooltipContent>
            <p>暂停选中的博客</p>
          </TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      <ConfirmDialog
        open={pausing}
        onOpenChange={setPausing}
        title={`暂停 ${selected.length} 个博客？`}
        desc='暂停后不再抓取，也不在前台展示。之后可以随时恢复。'
        confirmText='暂停'
        destructive
        isLoading={busy}
        handleConfirm={() => void setStatus('paused')}
      />
    </>
  )
}
