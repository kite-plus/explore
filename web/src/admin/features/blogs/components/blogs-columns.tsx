import { type ColumnDef } from '@tanstack/react-table'
import { cn } from '@/admin/lib/utils'
import { formatDate, formatDateTime } from '@/admin/lib/format'
import { Checkbox } from '@/admin/components/ui/checkbox'
import { BlogAvatar } from '@/admin/components/blog-avatar'
import { DataTableColumnHeader } from '@/admin/components/data-table'
import { TimeAgo } from '@/admin/components/time-ago'
import type { AdminBlog } from '@/lib/admin-types'
import { blogState, blogStates } from '../data/data'
import { DataTableRowActions } from './data-table-row-actions'
import { useBlogs } from './blogs-provider'

function BlogCell({ blog }: { blog: AdminBlog }) {
  const { setOpen, setCurrentRow } = useBlogs()
  return (
    <button
      type='button'
      className='flex w-full items-center gap-3 text-start'
      onClick={() => {
        setCurrentRow(blog)
        setOpen('detail')
      }}
    >
      <BlogAvatar host={blog.host} name={blog.name} />
      <span className='grid min-w-0'>
        <span className='truncate font-medium hover:underline'>
          {blog.name || blog.host}
        </span>
        <span className='truncate font-mono text-xs text-muted-foreground'>
          {blog.host}
        </span>
      </span>
    </button>
  )
}

export const blogsColumns: ColumnDef<AdminBlog>[] = [
  {
    id: 'select',
    header: ({ table }) => (
      <Checkbox
        checked={
          table.getIsAllPageRowsSelected() ||
          (table.getIsSomePageRowsSelected() && 'indeterminate')
        }
        onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
        aria-label='全选'
        className='translate-y-0.5'
      />
    ),
    cell: ({ row }) => (
      <Checkbox
        checked={row.getIsSelected()}
        onCheckedChange={(value) => row.toggleSelected(!!value)}
        aria-label='选择这一行'
        className='translate-y-0.5'
      />
    ),
    enableSorting: false,
    enableHiding: false,
  },
  {
    accessorKey: 'name',
    header: ({ column }) => <DataTableColumnHeader column={column} title='博客' />,
    cell: ({ row }) => <BlogCell blog={row.original} />,
    enableHiding: false,
    meta: { title: '博客', className: 'w-1/3 max-w-0 min-w-56' },
  },
  {
    id: 'status',
    accessorFn: blogState,
    header: ({ column }) => <DataTableColumnHeader column={column} title='状态' />,
    cell: ({ row }) => {
      const state = blogStates.find((item) => item.value === row.getValue('status'))
      if (!state) return null
      return (
        <div className='flex w-24 items-center gap-2'>
          <state.icon
            className={cn(
              'size-4 text-muted-foreground',
              state.value === 'unhealthy' && 'text-destructive'
            )}
          />
          <span>{state.label}</span>
        </div>
      )
    },
    filterFn: (row, id, value) => value.includes(row.getValue(id)),
    meta: { title: '状态' },
  },
  {
    id: 'generator',
    accessorFn: (blog) => blog.generator,
    header: ({ column }) => <DataTableColumnHeader column={column} title='程序' />,
    cell: ({ row }) => (
      <span className='text-muted-foreground'>{row.original.generator || '—'}</span>
    ),
    filterFn: (row, id, value) => value.includes(row.getValue(id)),
    meta: { title: '程序' },
  },
  {
    id: 'language',
    accessorFn: (blog) => blog.language,
    header: ({ column }) => <DataTableColumnHeader column={column} title='语言' />,
    cell: ({ row }) => (
      <span className='text-muted-foreground'>{row.original.language || '—'}</span>
    ),
    filterFn: (row, id, value) => value.includes(row.getValue(id)),
    meta: { title: '语言' },
  },
  {
    id: 'last_succeeded_at',
    accessorFn: (blog) => blog.last_succeeded_at ?? '',
    header: ({ column }) => <DataTableColumnHeader column={column} title='上次成功' />,
    cell: ({ row }) => (
      <TimeAgo iso={row.original.last_succeeded_at} className='text-nowrap text-muted-foreground' />
    ),
    meta: { title: '上次成功' },
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
    accessorKey: 'created_at',
    header: ({ column }) => <DataTableColumnHeader column={column} title='收录时间' />,
    cell: ({ row }) => (
      <span className='text-nowrap text-muted-foreground' title={formatDateTime(row.original.created_at)}>
        {formatDate(row.original.created_at)}
      </span>
    ),
    meta: { title: '收录时间' },
  },
  {
    id: 'actions',
    cell: ({ row }) => <DataTableRowActions row={row} />,
  },
]
