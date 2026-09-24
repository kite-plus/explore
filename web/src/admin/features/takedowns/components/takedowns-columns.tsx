import { type ColumnDef } from '@tanstack/react-table'
import { cn } from '@/admin/lib/utils'
import { Button } from '@/admin/components/ui/button'
import { DataTableColumnHeader } from '@/admin/components/data-table'
import { LongText } from '@/admin/components/long-text'
import { TimeAgo } from '@/admin/components/time-ago'
import type { Takedown } from '@/lib/admin-types'
import { statusOrder, takedownStatuses, targetTypes } from '../data/data'
import { targetName, useTakedowns } from './takedowns-provider'

function ReviewButton({ item }: { item: Takedown }) {
  const { setOpen, setCurrentRow } = useTakedowns()
  return (
    <Button
      variant={item.status === 'pending' ? 'outline' : 'ghost'}
      size='sm'
      className='h-8'
      onClick={() => {
        setCurrentRow(item)
        setOpen('review')
      }}
    >
      {item.status === 'pending' ? '处理' : '查看'}
    </Button>
  )
}

export const takedownsColumns: ColumnDef<Takedown>[] = [
  {
    id: 'target',
    accessorFn: targetName,
    header: ({ column }) => <DataTableColumnHeader column={column} title='对象' />,
    cell: ({ row }) => (
      <div className='grid min-w-0'>
        <LongText className='font-medium'>{targetName(row.original)}</LongText>
        {row.original.target_type === 'entry' && (
          <span className='truncate font-mono text-xs text-muted-foreground'>{row.original.blog_host}</span>
        )}
      </div>
    ),
    enableHiding: false,
    meta: { title: '对象', className: 'w-1/4 max-w-0 min-w-48' },
  },
  {
    accessorKey: 'target_type',
    header: ({ column }) => <DataTableColumnHeader column={column} title='类型' />,
    cell: ({ row }) => {
      const type = targetTypes.find((item) => item.value === row.getValue('target_type'))
      if (!type) return null
      return (
        <div className='flex items-center gap-2'>
          <type.icon className='size-4 text-muted-foreground' />
          <span>{type.label}</span>
        </div>
      )
    },
    filterFn: (row, id, value) => value.includes(row.getValue(id)),
    meta: { title: '类型' },
  },
  {
    accessorKey: 'reason',
    header: ({ column }) => <DataTableColumnHeader column={column} title='原因' />,
    cell: ({ row }) => <LongText className='max-w-80'>{row.original.reason}</LongText>,
    enableSorting: false,
    meta: { title: '原因', className: 'max-w-0 w-1/3 min-w-48' },
  },
  {
    id: 'requester',
    accessorFn: (item) => item.requester || '管理员',
    header: ({ column }) => <DataTableColumnHeader column={column} title='发起人' />,
    cell: ({ row }) => <LongText className='max-w-40 text-muted-foreground'>{row.getValue('requester')}</LongText>,
    meta: { title: '发起人' },
  },
  {
    accessorKey: 'created_at',
    header: ({ column }) => <DataTableColumnHeader column={column} title='提交时间' />,
    cell: ({ row }) => <TimeAgo iso={row.original.created_at} className='text-nowrap text-muted-foreground' />,
    meta: { title: '提交时间' },
  },
  {
    accessorKey: 'status',
    header: ({ column }) => <DataTableColumnHeader column={column} title='状态' />,
    cell: ({ row }) => {
      const status = takedownStatuses.find((item) => item.value === row.getValue('status'))
      if (!status) return null
      return (
        <div className={cn('flex w-20 items-center gap-2', status.className)}>
          <status.icon className='size-4' />
          <span>{status.label}</span>
        </div>
      )
    },
    sortingFn: (a, b) => statusOrder[a.original.status] - statusOrder[b.original.status],
    filterFn: (row, id, value) => value.includes(row.getValue(id)),
    meta: { title: '状态' },
  },
  {
    id: 'actions',
    cell: ({ row }) => <ReviewButton item={row.original} />,
    meta: { className: 'text-end' },
  },
]
