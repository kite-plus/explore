import { type Row } from '@tanstack/react-table'
import { Check, ExternalLink, MoreHorizontal, PanelRight, X } from 'lucide-react'
import { Button } from '@/admin/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/admin/components/ui/dropdown-menu'
import type { AdminSubmission } from '@/lib/admin-types'
import { useSubmissions } from './submissions-provider'

export function DataTableRowActions({ row }: { row: Row<AdminSubmission> }) {
  const submission = row.original
  const { setOpen, setCurrentRow } = useSubmissions()
  const pending = submission.status === 'pending'
  const open = (dialog: 'detail' | 'approve' | 'reject') => () => {
    setCurrentRow(submission)
    setOpen(dialog)
  }

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='flex h-8 w-8 p-0 data-[state=open]:bg-muted'>
          <MoreHorizontal className='h-4 w-4' />
          <span className='sr-only'>打开菜单</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-40'>
        <DropdownMenuItem onClick={open('detail')}>
          查看详情
          <DropdownMenuShortcut>
            <PanelRight size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <a href={submission.site_url} target='_blank' rel='noopener'>
            打开网站
            <DropdownMenuShortcut>
              <ExternalLink size={16} />
            </DropdownMenuShortcut>
          </a>
        </DropdownMenuItem>
        {pending && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={open('approve')}>
              通过
              <DropdownMenuShortcut>
                <Check size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuItem variant='destructive' onClick={open('reject')}>
              驳回
              <DropdownMenuShortcut>
                <X size={16} />
              </DropdownMenuShortcut>
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
