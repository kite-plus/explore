import { useEffect, useState } from 'react'
import {
  type SortingState,
  type VisibilityState,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useTableUrlState } from '@/admin/hooks/use-table-url-state'
import { useNavigate, useSearch } from '@/admin/router'
import { DataTablePagination, DataTableToolbar, DataTableView } from '@/admin/components/data-table'
import type { FetchQueueItem } from '@/lib/admin-types'
import { queueStatuses } from '../data/data'
import { queueColumns as columns } from './queue-columns'

export function QueueTable({ data, loading }: { data: FetchQueueItem[]; loading: boolean }) {
  const [sorting, setSorting] = useState<SortingState>([{ id: 'next_fetch_at', desc: false }])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({ last_fetched_at: false })
  const { globalFilter, onGlobalFilterChange, columnFilters, onColumnFiltersChange, pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: useSearch(),
      navigate: useNavigate(),
      pagination: { defaultPage: 1, defaultPageSize: 20 },
      globalFilter: { enabled: true, key: 'filter' },
      columnFilters: [{ columnId: 'status', searchKey: 'status', type: 'array' }],
    })

  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnVisibility, columnFilters, globalFilter, pagination },
    getRowId: (item) => item.host,
    // Polling refreshes the data; keep the reader on the page they are looking at.
    autoResetPageIndex: false,
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    globalFilterFn: (row, _columnId, filterValue) => {
      const search = String(filterValue).toLowerCase()
      return row.original.host.includes(search) || row.original.name.toLowerCase().includes(search)
    },
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    onPaginationChange,
    onGlobalFilterChange,
    onColumnFiltersChange,
  })

  const pageCount = table.getPageCount()
  useEffect(() => {
    if (!loading) ensurePageInRange(pageCount)
  }, [loading, pageCount, ensurePageInRange])

  return (
    <div className='flex flex-1 flex-col gap-4'>
      <DataTableToolbar
        table={table}
        searchPlaceholder='按域名或名称筛选…'
        filters={[{ columnId: 'status', title: '状态', options: queueStatuses }]}
      />
      <DataTableView table={table} loading={loading} emptyText='没有符合条件的抓取任务。' />
      <DataTablePagination table={table} className='mt-auto' />
    </div>
  )
}
