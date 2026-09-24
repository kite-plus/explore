import { type Table } from '@tanstack/react-table'
import { Eye, EyeOff } from 'lucide-react'
import { Button } from '@/admin/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/admin/components/ui/tooltip'
import { DataTableBulkActions as BulkActionsToolbar } from '@/admin/components/data-table'
import type { AdminEntryRow } from '@/lib/admin-types'
import { useSetHidden } from '../api'
import { useEntries } from './entries-provider'

export function DataTableBulkActions({ table }: { table: Table<AdminEntryRow> }) {
  const { setOpen, setTargets } = useEntries()
  const setHidden = useSetHidden()
  const selected = table.getFilteredSelectedRowModel().rows.map((row) => row.original)

  return (
    <BulkActionsToolbar table={table} entityName='文章'>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant='outline'
            size='icon'
            className='size-8'
            aria-label='恢复展示'
            onClick={() => {
              void setHidden(selected.filter((entry) => entry.hidden), false)
              table.resetRowSelection()
            }}
          >
            <Eye />
          </Button>
        </TooltipTrigger>
        <TooltipContent>
          <p>恢复展示</p>
        </TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant='destructive'
            size='icon'
            className='size-8'
            aria-label='隐藏'
            onClick={() => {
              setTargets(selected.filter((entry) => !entry.hidden))
              setOpen('hide')
            }}
          >
            <EyeOff />
          </Button>
        </TooltipTrigger>
        <TooltipContent>
          <p>隐藏选中的文章</p>
        </TooltipContent>
      </Tooltip>
    </BulkActionsToolbar>
  )
}
