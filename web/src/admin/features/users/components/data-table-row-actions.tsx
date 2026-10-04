import { Ban, Eye, KeyRound, LogOut, MoreHorizontal, Pencil, RotateCcw, ShieldCheck, ShieldOff, Trash2 } from 'lucide-react'
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
import { useIsSelf, useUsers } from './users-provider'

/** The account's actions, shared by the table rows and the detail drawer. */
export function UserActionsMenu({ user, children }: { user: AdminUserRow; children: React.ReactNode }) {
  const { act, showDetail, detailId } = useUsers()
  const self = useIsSelf()(user)
  const disabled = Boolean(user.disabled_at)

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>{children}</DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-44'>
        {detailId !== user.id && (
          <DropdownMenuItem onClick={() => showDetail(user.id)}>
            查看详情
            <DropdownMenuShortcut>
              <Eye size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={() => act('edit', [user])}>
          编辑资料
          <DropdownMenuShortcut>
            <Pencil size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        {!self && (
          <>
            <DropdownMenuItem onClick={() => act('reset-password', [user])}>
              重置密码
              <DropdownMenuShortcut>
                <KeyRound size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuItem disabled={disabled} onClick={() => act('admin', [user])}>
              {user.is_admin ? '取消后台权限' : '授予后台权限'}
              <DropdownMenuShortcut>
                {user.is_admin ? <ShieldOff size={16} /> : <ShieldCheck size={16} />}
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuItem disabled={disabled} onClick={() => act('sign-out', [user])}>
              强制下线
              <DropdownMenuShortcut>
                <LogOut size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            {disabled ? (
              <DropdownMenuItem onClick={() => act('restore', [user])}>
                恢复账号
                <DropdownMenuShortcut>
                  <RotateCcw size={16} />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
            ) : (
              <DropdownMenuItem variant='destructive' onClick={() => act('disable', [user])}>
                停用账号
                <DropdownMenuShortcut>
                  <Ban size={16} />
                </DropdownMenuShortcut>
              </DropdownMenuItem>
            )}
            <DropdownMenuItem variant='destructive' onClick={() => act('delete', [user])}>
              删除账号
              <DropdownMenuShortcut>
                <Trash2 size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function DataTableRowActions({ user }: { user: AdminUserRow }) {
  return (
    <UserActionsMenu user={user}>
      <Button variant='ghost' className='flex h-8 w-8 p-0 data-[state=open]:bg-muted'>
        <MoreHorizontal className='h-4 w-4' />
        <span className='sr-only'>打开菜单</span>
      </Button>
    </UserActionsMenu>
  )
}
