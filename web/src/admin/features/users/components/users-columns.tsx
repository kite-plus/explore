import { type ColumnDef } from '@tanstack/react-table'
import { cn } from '@/admin/lib/utils'
import { formatDate, formatDateTime } from '@/admin/lib/format'
import { DataTableColumnHeader } from '@/admin/components/data-table'
import { LongText } from '@/admin/components/long-text'
import { UserAvatar } from '@/admin/components/user-avatar'
import type { AdminUserRow } from '@/lib/admin-types'
import { roles, userStatuses } from '../data/data'
import { DataTableRowActions } from './data-table-row-actions'

export const usersColumns: ColumnDef<AdminUserRow>[] = [
  {
    accessorKey: 'display_name',
    header: ({ column }) => <DataTableColumnHeader column={column} title='用户' />,
    cell: ({ row }) => (
      <div className='flex items-center gap-3'>
        <UserAvatar name={row.original.display_name} email={row.original.email} />
        <LongText className='max-w-40 font-medium'>{row.original.display_name}</LongText>
      </div>
    ),
    enableSorting: false,
    enableHiding: false,
    meta: { title: '用户' },
  },
  {
    accessorKey: 'email',
    header: ({ column }) => <DataTableColumnHeader column={column} title='邮箱' />,
    cell: ({ row }) => <div className='w-fit text-nowrap'>{row.original.email}</div>,
    enableSorting: false,
    meta: { title: '邮箱' },
  },
  {
    id: 'status',
    accessorFn: (user) => (user.disabled_at ? 'disabled' : 'active'),
    header: ({ column }) => <DataTableColumnHeader column={column} title='状态' />,
    cell: ({ row }) => {
      const disabled = Boolean(row.original.disabled_at)
      const status = userStatuses[disabled ? 'disabled' : 'active']
      return (
        <div
          className={cn('flex items-center gap-2', status.className)}
          title={disabled ? `停用于 ${formatDateTime(row.original.disabled_at)}` : undefined}
        >
          <status.icon className='size-4' />
          <span className='text-nowrap'>{status.label}</span>
        </div>
      )
    },
    enableSorting: false,
    enableHiding: false,
    meta: { title: '状态' },
  },
  {
    id: 'role',
    accessorFn: (user) => (user.is_admin ? 'admin' : 'reader'),
    header: ({ column }) => <DataTableColumnHeader column={column} title='角色' />,
    cell: ({ row }) => {
      const role = roles.find(({ value }) => value === row.getValue('role'))
      if (!role) return null
      return (
        <div className='flex items-center gap-x-2'>
          <role.icon size={16} className='text-muted-foreground' />
          <span className='text-sm'>{role.label}</span>
        </div>
      )
    },
    enableSorting: false,
    meta: { title: '角色' },
  },
  {
    accessorKey: 'subscription_count',
    header: ({ column }) => <DataTableColumnHeader column={column} title='订阅' />,
    cell: ({ row }) => <span className='tabular-nums'>{row.original.subscription_count}</span>,
    enableSorting: false,
    meta: { title: '订阅' },
  },
  {
    accessorKey: 'owned_blog_count',
    header: ({ column }) => <DataTableColumnHeader column={column} title='认领博客' />,
    cell: ({ row }) => <span className='tabular-nums'>{row.original.owned_blog_count}</span>,
    enableSorting: false,
    meta: { title: '认领博客' },
  },
  {
    accessorKey: 'created_at',
    header: ({ column }) => <DataTableColumnHeader column={column} title='注册时间' />,
    cell: ({ row }) => (
      <span className='text-nowrap text-muted-foreground' title={formatDateTime(row.original.created_at)}>
        {formatDate(row.original.created_at)}
      </span>
    ),
    enableSorting: false,
    meta: { title: '注册时间' },
  },
  {
    id: 'actions',
    cell: ({ row }) => <DataTableRowActions row={row} />,
  },
]
