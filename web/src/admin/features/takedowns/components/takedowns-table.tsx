import { useEffect, useState } from 'react'
import {
  type SortingState,
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
import type { Takedown } from '@/lib/admin-types'
import { takedownStatuses, targetTypes } from '../data/data'
import { takedownsColumns as columns } from './takedowns-columns'
import { targetName } from './takedowns-provider'

export function TakedownsTable({ data, loading }: { data: Takedown[]; loading: boolean }) {
  // Pending first, newest first within each status.
  const [sorting, setSorting] = useState<SortingState>([
    { id: 'status', desc: false },
    { id: 'created_at', desc: true },
  ])
  const { globalFilter, onGlobalFilterChange, columnFilters, onColumnFiltersChange, pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: useSearch(),
      navigate: useNavigate(),
      pagination: { defaultPage: 1, defaultPageSize: 20 },
      globalFilter: { enabled: true, key: 'filter' },
      columnFilters: [
        { columnId: 'status', searchKey: 'status', type: 'array' },
        { columnId: 'target_type', searchKey: 'type', type: 'array' },
      ],
    })

  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnFilters, globalFilter, pagination },
    // Refetches after edits must not send the reader back to page one.
    autoResetPageIndex: false,
    getRowId: (item) => item.id,
    onSortingChange: setSorting,
    globalFilterFn: (row, _columnId, filterValue) => {
      const search = String(filterValue).toLowerCase()
      const item = row.original
      return (
        targetName(item).toLowerCase().includes(search) ||
        item.blog_host.includes(search) ||
        item.reason.toLowerCase().includes(search)
      )
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
        searchPlaceholder='按对象、域名或原因筛选…'
        filters={[
          { columnId: 'status', title: '状态', options: takedownStatuses },
          { columnId: 'target_type', title: '类型', options: targetTypes },
        ]}
      />
      <DataTableView table={table} loading={loading} emptyText='没有符合条件的下架申请。' />
      <DataTablePagination table={table} className='mt-auto' />
    </div>
  )
}
