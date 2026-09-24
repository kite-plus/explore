import { useEffect, useMemo, useState } from 'react'
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
import { cn } from '@/admin/lib/utils'
import { languageKey, languageLabel } from '@/admin/lib/language'
import { useTableUrlState } from '@/admin/hooks/use-table-url-state'
import { useNavigate, useSearch } from '@/admin/router'
import { DataTablePagination, DataTableToolbar, DataTableView } from '@/admin/components/data-table'
import { generatorInfo } from '@/admin/components/generator-label'
import type { AdminBlog } from '@/lib/admin-types'
import { blogStates, facetOptions } from '../data/data'
import { blogsColumns as columns } from './blogs-columns'
import { DataTableBulkActions } from './data-table-bulk-actions'

type BlogsTableProps = {
  data: AdminBlog[]
  loading: boolean
}

export function BlogsTable({ data, loading }: BlogsTableProps) {
  const [rowSelection, setRowSelection] = useState({})
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({
    language: false,
  })

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
      { columnId: 'generator', searchKey: 'generator', type: 'array' },
      { columnId: 'language', searchKey: 'language', type: 'array' },
    ],
  })

  const table = useReactTable({
    data,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      columnFilters,
      globalFilter,
      pagination,
    },
    // Refetches after edits must not send the reader back to page one.
    autoResetPageIndex: false,
    getRowId: (blog) => blog.host,
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    globalFilterFn: (row, _columnId, filterValue) => {
      const search = String(filterValue).toLowerCase()
      return (
        row.original.host.includes(search) ||
        row.original.name.toLowerCase().includes(search)
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

  const generators = useMemo(
    () =>
      facetOptions(data.map((blog) => blog.generator)).map(({ value }) => {
        const { label, icon } = generatorInfo(value)
        return { label, value, icon }
      }),
    [data]
  )
  const languages = useMemo(
    () =>
      facetOptions(data.map((blog) => languageKey(blog.language))).map(({ value }) => ({
        label: languageLabel(value),
        value,
      })),
    [data]
  )

  return (
    <div
      className={cn(
        'max-sm:has-[div[role="toolbar"]]:mb-16',
        'flex flex-1 flex-col gap-4'
      )}
    >
      <DataTableToolbar
        table={table}
        searchPlaceholder='按域名或名称筛选…'
        filters={[
          { columnId: 'status', title: '状态', options: blogStates },
          { columnId: 'generator', title: '程序', options: generators },
          { columnId: 'language', title: '语言', options: languages },
        ]}
      />
      <DataTableView table={table} loading={loading} emptyText='没有符合条件的博客。' />
      <DataTablePagination table={table} className='mt-auto' />
      <DataTableBulkActions table={table} />
    </div>
  )
}
