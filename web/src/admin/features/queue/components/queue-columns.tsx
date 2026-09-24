import { type ColumnDef } from '@tanstack/react-table'
import { History, MoreHorizontal, PanelRight, RefreshCw } from 'lucide-react'
import { cn } from '@/admin/lib/utils'
import { Link } from '@/admin/router'
import { Button } from '@/admin/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/admin/components/ui/dropdown-menu'
import { BlogAvatar } from '@/admin/components/blog-avatar'
import { DataTableColumnHeader } from '@/admin/components/data-table'
import { LongText } from '@/admin/components/long-text'
import { TimeAgo } from '@/admin/components/time-ago'
import type { FetchQueueItem } from '@/lib/admin-types'
import { useFetchNow } from '@/admin/features/blogs/api'
import { queueStatuses } from '../data/data'
import { useQueue } from './queue-provider'

function RowActions({ item }: { item: FetchQueueItem }) {
  const { setAttemptsFor } = useQueue()
  const fetchNow = useFetchNow()
  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='flex h-8 w-8 p-0 data-[state=open]:bg-muted'>
          <MoreHorizontal className='h-4 w-4' />
          <span className='sr-only'>打开菜单</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-40'>
        <DropdownMenuItem onClick={() => setAttemptsFor(item)}>
          执行记录
          <DropdownMenuShortcut>
            <History size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem disabled={item.queue_status === 'paused'} onClick={() => void fetchNow([item.host])}>
          立即抓取
          <DropdownMenuShortcut>
            <RefreshCw size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to={`/admin/blogs?filter=${encodeURIComponent(item.host)}`}>
            博客资料
            <DropdownMenuShortcut>
              <PanelRight size={16} />
            </DropdownMenuShortcut>
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export const queueColumns: ColumnDef<FetchQueueItem>[] = [
  {
    accessorKey: 'name',
    header: ({ column }) => <DataTableColumnHeader column={column} title='博客' />,
    cell: ({ row }) => (
      <div className='flex items-center gap-3'>
        <BlogAvatar host={row.original.host} name={row.original.name} />
        <span className='grid min-w-0'>
          <span className='truncate font-medium'>{row.original.name || row.original.host}</span>
          <span className='truncate font-mono text-xs text-muted-foreground'>{row.original.host}</span>
        </span>
      </div>
    ),
    enableHiding: false,
    meta: { title: '博客', className: 'w-1/4 max-w-0 min-w-56' },
  },
  {
    id: 'status',
    accessorFn: (item) => item.queue_status,
    header: ({ column }) => <DataTableColumnHeader column={column} title='状态' />,
    cell: ({ row }) => {
      const status = queueStatuses.find((item) => item.value === row.getValue('status'))
      if (!status) return null
      return (
        <div className={cn('flex w-24 items-center gap-2', status.className)}>
          <status.icon className='size-4' />
          <span>{status.label}</span>
        </div>
      )
    },
    filterFn: (row, id, value) => value.includes(row.getValue(id)),
    meta: { title: '状态' },
  },
  {
    accessorKey: 'consecutive_failures',
    header: ({ column }) => <DataTableColumnHeader column={column} title='连续失败' />,
    cell: ({ row }) => {
      const failures = row.original.consecutive_failures
      return (
        <span className={cn('tabular-nums', failures > 0 ? 'font-medium text-destructive' : 'text-muted-foreground')}>
          {failures}
        </span>
      )
    },
    meta: { title: '连续失败' },
  },
  {
    id: 'error',
    accessorFn: (item) => item.last_attempt?.error || item.last_error,
    header: ({ column }) => <DataTableColumnHeader column={column} title='最近错误' />,
    cell: ({ row }) => {
      const error = row.getValue<string>('error')
      return error ? (
        <LongText className='max-w-64 text-destructive'>{error}</LongText>
      ) : (
        <span className='text-muted-foreground'>—</span>
      )
    },
    enableSorting: false,
    meta: { title: '最近错误', className: 'max-w-0 w-1/4 min-w-40' },
  },
  {
    accessorKey: 'next_fetch_at',
    header: ({ column }) => <DataTableColumnHeader column={column} title='下次调度' />,
    cell: ({ row }) => <TimeAgo iso={row.original.next_fetch_at} className='text-nowrap text-muted-foreground' />,
    meta: { title: '下次调度' },
  },
  {
    id: 'last_fetched_at',
    accessorFn: (item) => item.last_fetched_at ?? '',
    header: ({ column }) => <DataTableColumnHeader column={column} title='上次执行' />,
    cell: ({ row }) => <TimeAgo iso={row.original.last_fetched_at} className='text-nowrap text-muted-foreground' />,
    meta: { title: '上次执行' },
  },
  {
    id: 'last_succeeded_at',
    accessorFn: (item) => item.last_succeeded_at ?? '',
    header: ({ column }) => <DataTableColumnHeader column={column} title='上次成功' />,
    cell: ({ row }) => <TimeAgo iso={row.original.last_succeeded_at} className='text-nowrap text-muted-foreground' />,
    meta: { title: '上次成功' },
  },
  {
    id: 'actions',
    cell: ({ row }) => <RowActions item={row.original} />,
  },
]
