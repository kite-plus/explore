import { type Row } from '@tanstack/react-table'
import { ExternalLink, MoreHorizontal, PanelRight, Pencil, RefreshCw, Trash2 } from 'lucide-react'
import { Button } from '@/admin/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/admin/components/ui/dropdown-menu'
import type { AdminBlog } from '@/lib/admin-types'
import { useFetchNow } from '../api'
import { useBlogs } from './blogs-provider'

export function DataTableRowActions({ row }: { row: Row<AdminBlog> }) {
  const blog = row.original
  const { setOpen, setCurrentRow } = useBlogs()
  const fetchNow = useFetchNow()

  return (
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='flex h-8 w-8 p-0 data-[state=open]:bg-muted'>
          <MoreHorizontal className='h-4 w-4' />
          <span className='sr-only'>打开菜单</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-44'>
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(blog)
            setOpen('detail')
          }}
        >
          查看详情
          <DropdownMenuShortcut>
            <PanelRight size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setCurrentRow(blog)
            setOpen('edit')
          }}
        >
          编辑资料
          <DropdownMenuShortcut>
            <Pencil size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem
          disabled={blog.status === 'paused'}
          onClick={() => void fetchNow([blog.host])}
        >
          立即抓取
          <DropdownMenuShortcut>
            <RefreshCw size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <a href={blog.site_url} target='_blank' rel='noopener'>
            访问博客
            <DropdownMenuShortcut>
              <ExternalLink size={16} />
            </DropdownMenuShortcut>
          </a>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant='destructive'
          onClick={() => {
            setCurrentRow(blog)
            setOpen('delete')
          }}
        >
          移除
          <DropdownMenuShortcut>
            <Trash2 size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
