import { type ColumnDef } from '@tanstack/react-table'
import { cn } from '@/admin/lib/utils'
import { formatDate, formatDateTime } from '@/admin/lib/format'
import { Badge } from '@/admin/components/ui/badge'
import { Checkbox } from '@/admin/components/ui/checkbox'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/admin/components/ui/tooltip'
import { DataTableColumnHeader } from '@/admin/components/data-table'
import { TimeAgo } from '@/admin/components/time-ago'
import { UserAvatar } from '@/admin/components/user-avatar'
import type { AdminUserRow } from '@/lib/admin-types'
import { userRole, userStatus } from '../data/data'
import { DataTableRowActions } from './data-table-row-actions'
import { useIsSelf, useUsers } from './users-provider'

function UserCell({ user }: { user: AdminUserRow }) {
  const { showDetail } = useUsers()
  const isSelf = useIsSelf()
  return (
    <button type='button' className='flex w-full items-center gap-3 text-start' onClick={() => showDetail(user.id)}>
      <UserAvatar name={user.display_name} email={user.email} />
      <span className='grid min-w-0'>
        <span className='flex items-center gap-2'>
          <span className='truncate font-medium hover:underline'>{user.display_name}</span>
          {isSelf(user) && (
            <Badge variant='secondary' className='px-1.5 py-0 font-normal'>
              你
            </Badge>
          )}
        </span>
        <span className='truncate text-xs text-muted-foreground'>{user.email}</span>
      </span>
    </button>
  )
}

function StatusCell({ user }: { user: AdminUserRow }) {
  const status = userStatus(user)
  const label = (
    <span className={cn('flex w-fit items-center gap-2', status.className)}>
      <status.icon className='size-4' />
      <span className='text-nowrap'>{status.label}</span>
    </span>
  )
  if (!user.disabled_at) return label
  return (
    <Tooltip>
      <TooltipTrigger asChild>{label}</TooltipTrigger>
      <TooltipContent className='max-w-xs'>
        <p>{user.disabled_reason || '没有记录原因'}</p>
        <p className='opacity-70'>
          {formatDateTime(user.disabled_at)}
          {user.disabled_by && ` · ${user.disabled_by}`}
        </p>
      </TooltipContent>
    </Tooltip>
  )
}

const count = (value: number) => (
  <span className={cn('tabular-nums', value === 0 && 'text-muted-foreground')}>{value}</span>
)

export const usersColumns: ColumnDef<AdminUserRow>[] = [
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
    meta: { className: 'w-10' },
  },
  {
    id: 'user',
    header: ({ column }) => <DataTableColumnHeader column={column} title='用户' />,
    cell: ({ row }) => <UserCell user={row.original} />,
    enableSorting: false,
    enableHiding: false,
    meta: { title: '用户', className: 'max-w-72' },
  },
  {
    id: 'status',
    accessorFn: (user) => userStatus(user).value,
    header: ({ column }) => <DataTableColumnHeader column={column} title='状态' />,
    cell: ({ row }) => <StatusCell user={row.original} />,
    enableSorting: false,
    meta: { title: '状态' },
  },
  {
    id: 'role',
    accessorFn: (user) => userRole(user).value,
    header: ({ column }) => <DataTableColumnHeader column={column} title='角色' />,
    cell: ({ row }) => {
      const role = userRole(row.original)
      return (
        <div className='flex items-center gap-x-2'>
          <role.icon size={16} className='text-muted-foreground' />
          <span className='text-sm text-nowrap'>{role.label}</span>
        </div>
      )
    },
    enableSorting: false,
    meta: { title: '角色' },
  },
  {
    accessorKey: 'subscription_count',
    header: ({ column }) => <DataTableColumnHeader column={column} title='订阅' />,
    cell: ({ row }) => count(row.original.subscription_count),
    sortDescFirst: true,
    meta: { title: '订阅' },
  },
  {
    accessorKey: 'owned_blog_count',
    header: ({ column }) => <DataTableColumnHeader column={column} title='认领博客' />,
    cell: ({ row }) => count(row.original.owned_blog_count),
    sortDescFirst: true,
    meta: { title: '认领博客' },
  },
  {
    accessorKey: 'last_seen_at',
    header: ({ column }) => <DataTableColumnHeader column={column} title='最近活跃' />,
    cell: ({ row }) =>
      row.original.last_seen_at ? (
        <TimeAgo iso={row.original.last_seen_at} className='text-nowrap' />
      ) : (
        <span className='text-nowrap text-muted-foreground'>从未</span>
      ),
    sortDescFirst: true,
    meta: { title: '最近活跃' },
  },
  {
    accessorKey: 'created_at',
    header: ({ column }) => <DataTableColumnHeader column={column} title='注册时间' />,
    cell: ({ row }) => (
      <span className='text-nowrap text-muted-foreground' title={formatDateTime(row.original.created_at)}>
        {formatDate(row.original.created_at)}
      </span>
    ),
    sortDescFirst: true,
    meta: { title: '注册时间' },
  },
  {
    id: 'actions',
    cell: ({ row }) => <DataTableRowActions user={row.original} />,
    meta: { className: 'w-12' },
  },
]
