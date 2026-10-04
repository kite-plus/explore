import { type Table } from '@tanstack/react-table'
import { Ban, LogOut, RotateCcw } from 'lucide-react'
import { Button } from '@/admin/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/admin/components/ui/tooltip'
import { DataTableBulkActions as BulkActionsToolbar } from '@/admin/components/data-table'
import type { AdminUserRow } from '@/lib/admin-types'
import { type UsersDialogType, useIsSelf, useUsers } from './users-provider'

export function DataTableBulkActions({ table }: { table: Table<AdminUserRow> }) {
  const { act } = useUsers()
  const isSelf = useIsSelf()
  const selected = table.getFilteredSelectedRowModel().rows.map((row) => row.original)
  // The signed-in admin is left out: the API refuses changes to their own account.
  const enabled = selected.filter((user) => !user.disabled_at && !isSelf(user))
  const disabled = selected.filter((user) => user.disabled_at)

  const actions: {
    dialog: UsersDialogType
    targets: AdminUserRow[]
    label: string
    icon: React.ReactNode
    destructive?: boolean
  }[] = [
    { dialog: 'sign-out', targets: enabled, label: '强制下线', icon: <LogOut /> },
    { dialog: 'restore', targets: disabled, label: '恢复账号', icon: <RotateCcw /> },
    { dialog: 'disable', targets: enabled, label: '停用账号', icon: <Ban />, destructive: true },
  ]

  return (
    <BulkActionsToolbar table={table} entityName='账号'>
      {actions.map(({ dialog, targets, label, icon, destructive }) => (
        <Tooltip key={dialog}>
          <TooltipTrigger asChild>
            {/* A span keeps the tooltip working on a disabled button. */}
            <span tabIndex={targets.length ? undefined : 0}>
              <Button
                variant={destructive ? 'destructive' : 'outline'}
                size='icon'
                className='size-8'
                aria-label={label}
                disabled={targets.length === 0}
                onClick={() => act(dialog, targets, { onDone: () => table.resetRowSelection() })}
              >
                {icon}
              </Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>
              {targets.length
                ? `${label}（${targets.length} 个）`
                : `${label}：选中的账号都不适用`}
            </p>
          </TooltipContent>
        </Tooltip>
      ))}
    </BulkActionsToolbar>
  )
}
