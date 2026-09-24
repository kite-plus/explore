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
import type { AdminSubmission } from '@/lib/admin-types'
import { checkResults, submissionStatuses } from '../data/data'
import { submissionsColumns as columns } from './submissions-columns'

export function SubmissionsTable({ data, loading }: { data: AdminSubmission[]; loading: boolean }) {
  // Pending first, newest first within each status.
  const [sorting, setSorting] = useState<SortingState>([
    { id: 'status', desc: false },
    { id: 'created_at', desc: true },
  ])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({ reviewed_at: false })

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: useSearch(),
    navigate: useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: 'check', searchKey: 'check', type: 'array' },
    ],
  })

  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnVisibility, columnFilters, globalFilter, pagination },
    // Refetches after edits must not send the reader back to page one.
    autoResetPageIndex: false,
    getRowId: (submission) => submission.id,
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    globalFilterFn: (row, _columnId, filterValue) => {
      const search = String(filterValue).toLowerCase()
      return (
        row.original.host.includes(search) ||
        (row.original.check_report?.title ?? '').toLowerCase().includes(search)
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
        searchPlaceholder='按域名或名称筛选…'
        filters={[
          { columnId: 'status', title: '状态', options: submissionStatuses },
          { columnId: 'check', title: '检查', options: checkResults },
        ]}
      />
      <DataTableView table={table} loading={loading} emptyText='没有符合条件的提交。' />
      <DataTablePagination table={table} className='mt-auto' />
    </div>
  )
}
