import { useEffect, useState } from 'react'
import {
  type RowSelectionState,
  type VisibilityState,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { cn } from '@/admin/lib/utils'
import { useAdminQuery } from '@/admin/lib/api'
import { useDebounced } from '@/admin/hooks/use-debounced'
import { useTableUrlState } from '@/admin/hooks/use-table-url-state'
import { useNavigate, useSearch } from '@/admin/router'
import { DataTablePagination, DataTableToolbar, DataTableView } from '@/admin/components/data-table'
import { QueryError } from '@/admin/components/query-error'
import type { AdminEntryRow, Paged } from '@/lib/admin-types'
import { DataTableBulkActions } from './data-table-bulk-actions'
import { entriesColumns as columns } from './entries-columns'
import { HideDialog } from './hide-dialog'
import { useEntries } from './entries-provider'

/** Entries are paged and searched by the API, so the table only shows one page. */
export function EntriesTable() {
  const { open, setOpen, targets } = useEntries()
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})

  const { globalFilter, onGlobalFilterChange, pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: useSearch(),
      navigate: useNavigate(),
      pagination: { defaultPage: 1, defaultPageSize: 20 },
      globalFilter: { enabled: true, key: 'filter' },
    })
  const query = useDebounced(globalFilter ?? '')
  const path = `/entries?limit=${pagination.pageSize}&offset=${pagination.pageIndex * pagination.pageSize}&q=${encodeURIComponent(query)}`
  const { data, isPending, error, refetch } = useAdminQuery<Paged<AdminEntryRow>>(path, {
    keepPrevious: true,
  })
  const pageCount = Math.max(1, Math.ceil((data?.total ?? 0) / pagination.pageSize))

  const table = useReactTable({
    data: data?.data ?? [],
    columns,
    state: { rowSelection, columnVisibility, globalFilter, pagination },
    getRowId: (entry) => String(entry.id),
    enableRowSelection: true,
    manualPagination: true,
    manualFiltering: true,
    pageCount,
    onRowSelectionChange: setRowSelection,
    onColumnVisibilityChange: setColumnVisibility,
    onPaginationChange,
    onGlobalFilterChange,
    getCoreRowModel: getCoreRowModel(),
  })

  useEffect(() => {
    if (data) ensurePageInRange(pageCount)
  }, [data, pageCount, ensurePageInRange])

  // A new page or search starts with nothing selected.
  useEffect(() => setRowSelection({}), [path])

  if (error) return <QueryError message={error.message} onRetry={() => void refetch()} />

  return (
    <div className={cn('max-sm:has-[div[role="toolbar"]]:mb-16', 'flex flex-1 flex-col gap-4')}>
      <DataTableToolbar table={table} searchPlaceholder='按标题或博客域名搜索…' />
      <DataTableView
        table={table}
        loading={isPending}
        emptyText={query ? `没有标题或域名包含“${query}”的文章。` : '还没有缓存的文章。'}
      />
      <div className='flex items-center justify-between gap-4'>
        <p className='text-sm text-nowrap text-muted-foreground tabular-nums'>
          共 {data?.total.toLocaleString('zh-CN') ?? '…'} 篇
        </p>
      </div>
      <DataTablePagination table={table} className='mt-auto' />
      <DataTableBulkActions table={table} />
      <HideDialog
        open={open === 'hide'}
        onOpenChange={() => setOpen('hide')}
        entries={targets}
        onDone={() => table.resetRowSelection()}
      />
    </div>
  )
}
