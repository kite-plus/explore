import { type Row } from '@tanstack/react-table'
import { Ban, MoreHorizontal, RotateCcw, ShieldCheck, ShieldOff } from 'lucide-react'
import { useAuth } from '@/admin/context/auth-provider'
import { Button } from '@/admin/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/admin/components/ui/dropdown-menu'
import type { AdminUserRow } from '@/lib/admin-types'
import { useUsers } from './users-provider'

export function DataTableRowActions({ row }: { row: Row<AdminUserRow> }) {
  const user = row.original
  const { setPending } = useUsers()
  const { user: me } = useAuth()
  // The API refuses changes to one's own account.
  const self = me?.email === user.email

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='flex h-8 w-8 p-0 data-[state=open]:bg-muted' disabled={self}>
          <MoreHorizontal className='h-4 w-4' />
          <span className='sr-only'>打开菜单</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-40'>
        <DropdownMenuItem
          disabled={Boolean(user.disabled_at)}
          onClick={() => setPending({ user, change: { field: 'is_admin', value: !user.is_admin } })}
        >
          {user.is_admin ? '取消后台权限' : '授予后台权限'}
          <DropdownMenuShortcut>
            {user.is_admin ? <ShieldOff size={16} /> : <ShieldCheck size={16} />}
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant={user.disabled_at ? 'default' : 'destructive'}
          onClick={() => setPending({ user, change: { field: 'disabled', value: !user.disabled_at } })}
        >
          {user.disabled_at ? '恢复账号' : '停用账号'}
          <DropdownMenuShortcut>
            {user.disabled_at ? <RotateCcw size={16} /> : <Ban size={16} />}
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
