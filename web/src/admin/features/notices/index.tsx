import { useState } from 'react'
import { Pencil, Pin, Plus, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { toastError, useAdminMutation, useAdminQuery } from '@/admin/lib/api'
import { formatDateTime } from '@/admin/lib/format'
import { cn } from '@/admin/lib/utils'
import { Badge } from '@/admin/components/ui/badge'
import { Button } from '@/admin/components/ui/button'
import { Skeleton } from '@/admin/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/admin/components/ui/table'
import { ConfirmDialog } from '@/admin/components/confirm-dialog'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { QueryError } from '@/admin/components/query-error'
import type { AdminNotice } from '@/lib/admin-types'
import { NoticeDialog } from './notice-dialog'

const audiences = { '': '所有人', zh: '中文界面', en: '英文界面' }

/** Whether readers see it now, by the same rules the API uses. */
function status(n: AdminNotice, now = Date.now()) {
  if (!n.enabled) return { label: '草稿', className: 'text-muted-foreground' }
  if (n.starts_at && new Date(n.starts_at).getTime() > now) return { label: '未开始', className: 'text-info' }
  if (n.ends_at && new Date(n.ends_at).getTime() <= now) return { label: '已结束', className: 'text-muted-foreground' }
  return { label: '显示中', className: 'text-success' }
}

export function Notices() {
  const { data, isPending, error, refetch } = useAdminQuery<{ data: AdminNotice[] }>('/notices')
  const [editing, setEditing] = useState<AdminNotice | null>(null)
  const [creating, setCreating] = useState(false)
  const [deleting, setDeleting] = useState<AdminNotice | null>(null)
  const remove = useAdminMutation((id: number) => ({ path: `/notices/${id}`, method: 'DELETE' }))

  async function confirmDelete() {
    if (!deleting) return
    try {
      await remove.mutateAsync(deleting.id)
      toast.success(`已删除「${deleting.title}」`)
      setDeleting(null)
    } catch (cause) {
      toastError(cause)
    }
  }

  const notices = data?.data ?? []
  return (
    <>
      <AppHeader />
      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <div className='flex flex-wrap items-end justify-between gap-4'>
          <PageTitle
            title='公告与推广'
            description='Explore 自己发布的公告和广告，按位置插在最新流第一页的文章之间。广告总带「广告」标记，不统计曝光和点击。'
          />
          <Button onClick={() => setCreating(true)}>
            <Plus />
            新建
          </Button>
        </div>
        {error ? (
          <QueryError message={error.message} onRetry={() => void refetch()} />
        ) : isPending ? (
          <Skeleton className='h-40 w-full' />
        ) : notices.length === 0 ? (
          <p className='rounded-lg border border-dashed py-16 text-center text-sm text-muted-foreground'>
            还没有公告或广告。新建一条，发布后就会出现在最新流里。
          </p>
        ) : (
          <div className='overflow-hidden rounded-md border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>类型</TableHead>
                  <TableHead>标题</TableHead>
                  <TableHead>位置</TableHead>
                  <TableHead>给谁看</TableHead>
                  <TableHead>显示时间</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className='text-end'>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {notices.map((n) => {
                  const s = status(n)
                  return (
                    <TableRow key={n.id}>
                      <TableCell>
                        <Badge
                          variant='outline'
                          className={cn('rounded-full', n.kind === 'notice' && 'border-transparent bg-warning/12 text-warning')}
                        >
                          {n.kind === 'ad' ? '广告' : '公告'}
                        </Badge>
                      </TableCell>
                      <TableCell className='max-w-80'>
                        <div className='truncate font-medium'>{n.title}</div>
                        <div className='truncate text-xs text-muted-foreground'>
                          {n.source_name}
                          {n.url ? ` · ${n.url}` : ' · 条目页'}
                        </div>
                      </TableCell>
                      <TableCell className='text-nowrap'>
                        {n.position === 0 ? (
                          <span className='inline-flex items-center gap-1'>
                            <Pin className='size-3.5' />
                            置顶
                          </span>
                        ) : (
                          `第 ${n.position} 篇后`
                        )}
                      </TableCell>
                      <TableCell className='text-nowrap'>{audiences[n.audience]}</TableCell>
                      <TableCell className='text-nowrap text-xs text-muted-foreground'>
                        {n.starts_at ? formatDateTime(n.starts_at) : '立即'}
                        {' → '}
                        {n.ends_at ? formatDateTime(n.ends_at) : '一直'}
                      </TableCell>
                      <TableCell className={cn('text-nowrap', s.className)}>{s.label}</TableCell>
                      <TableCell className='text-end text-nowrap'>
                        <Button variant='ghost' size='sm' className='h-8' onClick={() => setEditing(n)}>
                          <Pencil />
                          编辑
                        </Button>
                        <Button variant='ghost' size='sm' className='h-8 text-destructive' onClick={() => setDeleting(n)}>
                          <Trash2 />
                          删除
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </Main>
      <NoticeDialog
        open={creating || editing !== null}
        onOpenChange={(open) => {
          if (!open) {
            setCreating(false)
            setEditing(null)
          }
        }}
        notice={editing}
      />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => !open && !remove.isPending && setDeleting(null)}
        title={`删除「${deleting?.title ?? ''}」？`}
        desc='删除后不能恢复。只想暂时不显示的话，编辑它并关掉「发布」。'
        confirmText='删除'
        destructive
        isLoading={remove.isPending}
        handleConfirm={() => void confirmDelete()}
        className='sm:max-w-md'
      />
    </>
  )
}
