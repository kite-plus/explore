import { type Row } from '@tanstack/react-table'
import { ExternalLink, Eye, EyeOff, MoreHorizontal } from 'lucide-react'
import { Button } from '@/admin/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/admin/components/ui/dropdown-menu'
import type { AdminEntryRow } from '@/lib/admin-types'
import { useSetHidden } from '../api'
import { useEntries } from './entries-provider'

export function DataTableRowActions({ row }: { row: Row<AdminEntryRow> }) {
  const entry = row.original
  const { setOpen, setTargets } = useEntries()
  const setHidden = useSetHidden()

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='flex h-8 w-8 p-0 data-[state=open]:bg-muted'>
          <MoreHorizontal className='h-4 w-4' />
          <span className='sr-only'>打开菜单</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-40'>
        <DropdownMenuItem asChild>
          <a href={entry.url} target='_blank' rel='noopener'>
            打开原文
            <DropdownMenuShortcut>
              <ExternalLink size={16} />
            </DropdownMenuShortcut>
          </a>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        {entry.hidden ? (
          <DropdownMenuItem onClick={() => void setHidden([entry], false)}>
            恢复展示
            <DropdownMenuShortcut>
              <Eye size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem
            variant='destructive'
            onClick={() => {
              setTargets([entry])
              setOpen('hide')
            }}
          >
            隐藏文章
            <DropdownMenuShortcut>
              <EyeOff size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
