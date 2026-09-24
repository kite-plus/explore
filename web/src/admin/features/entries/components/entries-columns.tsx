import { type ColumnDef } from '@tanstack/react-table'
import { Eye, EyeOff } from 'lucide-react'
import { formatDate, formatDateTime } from '@/admin/lib/format'
import { Link } from '@/admin/router'
import { Checkbox } from '@/admin/components/ui/checkbox'
import { DataTableColumnHeader } from '@/admin/components/data-table'
import { LongText } from '@/admin/components/long-text'
import type { AdminEntryRow } from '@/lib/admin-types'
import { DataTableRowActions } from './data-table-row-actions'

export const entriesColumns: ColumnDef<AdminEntryRow>[] = [
  {
    id: 'select',
    header: ({ table }) => (
      <Checkbox
        checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
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
    accessorKey: 'title',
    header: ({ column }) => <DataTableColumnHeader column={column} title='文章' />,
    cell: ({ row }) => {
      const entry = row.original
      return (
        <div className='grid min-w-0'>
          <a href={entry.url} target='_blank' rel='noopener' className='truncate font-medium hover:underline' title={entry.title}>
            {entry.title}
          </a>
          <span className='truncate text-xs text-muted-foreground'>
            #{entry.id}
            {entry.tags?.length ? ` · ${entry.tags.join(' · ')}` : ''}
          </span>
        </div>
      )
    },
    enableSorting: false,
    enableHiding: false,
    meta: { title: '文章', className: 'w-1/2 max-w-0 min-w-64' },
  },
  {
    id: 'blog',
    accessorFn: (entry) => entry.blog_host,
    header: ({ column }) => <DataTableColumnHeader column={column} title='博客' />,
    cell: ({ row }) => (
      <div className='grid max-w-48 min-w-0'>
        <LongText className='max-w-48'>{row.original.blog_name}</LongText>
        <Link
          to={`/admin/blogs?filter=${encodeURIComponent(row.original.blog_host)}`}
          className='truncate font-mono text-xs text-muted-foreground hover:underline'
        >
          {row.original.blog_host}
        </Link>
      </div>
    ),
    enableSorting: false,
    meta: { title: '博客' },
  },
  {
    id: 'published_at',
    accessorFn: (entry) => entry.published_at ?? '',
    header: ({ column }) => <DataTableColumnHeader column={column} title='发布时间' />,
    cell: ({ row }) => (
      <span className='text-nowrap text-muted-foreground' title={formatDateTime(row.original.published_at)}>
        {formatDate(row.original.published_at)}
      </span>
    ),
    enableSorting: false,
    meta: { title: '发布时间' },
  },
  {
    id: 'status',
    accessorFn: (entry) => (entry.hidden ? 'hidden' : 'visible'),
    header: ({ column }) => <DataTableColumnHeader column={column} title='状态' />,
    cell: ({ row }) =>
      row.original.hidden ? (
        <div className='flex items-center gap-2 text-destructive' title={row.original.hide_reason}>
          <EyeOff className='size-4' />
          <span className='text-nowrap'>已隐藏</span>
        </div>
      ) : (
        <div className='flex items-center gap-2'>
          <Eye className='size-4 text-muted-foreground' />
          <span>公开</span>
        </div>
      ),
    enableSorting: false,
    meta: { title: '状态' },
  },
  {
    id: 'actions',
    cell: ({ row }) => <DataTableRowActions row={row} />,
  },
]
