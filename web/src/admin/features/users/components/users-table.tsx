import { useEffect, useState } from 'react'
import {
  type RowSelectionState,
  type SortingState,
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
import type { AdminUserCounts, AdminUsersPage } from '@/lib/admin-types'
import { roles, sortKeys, userStatuses } from '../data/data'
import { DataTableBulkActions } from './data-table-bulk-actions'
import { usersColumns as columns } from './users-columns'

const defaultSort = { id: 'created_at', desc: true }

/** The API takes one value per filter; none or both mean no filter. */
function single(filters: { id: string; value: unknown }[], id: string) {
  const value = filters.find((filter) => filter.id === id)?.value
  return Array.isArray(value) && value.length === 1 ? String(value[0]) : ''
}

function facetCounts(counts: AdminUserCounts | undefined) {
  if (!counts) return undefined
  return {
    status: { active: counts.active, disabled: counts.disabled },
    role: { admin: counts.admin, reader: counts.reader },
  }
}

/** Users are paged, filtered and sorted by the API, so the table only shows one page. */
export function UsersTable() {
  const search = useSearch()
  const navigate = useNavigate()
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search,
    navigate,
    pagination: { defaultPage: 1, defaultPageSize: 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: 'role', searchKey: 'role', type: 'array' },
    ],
  })

  const sortId = typeof search.sort === 'string' && search.sort in sortKeys ? search.sort : defaultSort.id
  const sorting: SortingState = [{ id: sortId, desc: search.order !== 'asc' }]
  const onSortingChange = (updater: SortingState | ((old: SortingState) => SortingState)) => {
    const [next = defaultSort] = typeof updater === 'function' ? updater(sorting) : updater
    const isDefault = next.id === defaultSort.id && next.desc
    navigate({
      search: (prev) => ({
        ...prev,
        page: undefined,
        sort: isDefault ? undefined : next.id,
        order: isDefault ? undefined : next.desc ? 'desc' : 'asc',
      }),
    })
  }

  const query = useDebounced(globalFilter ?? '')
  const params = new URLSearchParams({
    limit: String(pagination.pageSize),
    offset: String(pagination.pageIndex * pagination.pageSize),
    q: query,
    status: single(columnFilters, 'status'),
    role: single(columnFilters, 'role'),
    sort: sortKeys[sortId],
    order: sorting[0].desc ? 'desc' : 'asc',
  })
  const path = `/users?${params}`
  const { data, isPending, error, refetch } = useAdminQuery<AdminUsersPage>(path, { keepPrevious: true })
  const pageCount = Math.max(1, Math.ceil((data?.total ?? 0) / pagination.pageSize))

  const table = useReactTable({
    data: data?.data ?? [],
    columns,
    state: { rowSelection, columnVisibility, columnFilters, globalFilter, pagination, sorting },
    getRowId: (user) => user.id,
    enableRowSelection: true,
    enableSortingRemoval: false,
    manualPagination: true,
    manualFiltering: true,
    manualSorting: true,
    pageCount,
    onRowSelectionChange: setRowSelection,
    onColumnVisibilityChange: setColumnVisibility,
    onColumnFiltersChange,
    onSortingChange,
    onPaginationChange,
    onGlobalFilterChange,
    getCoreRowModel: getCoreRowModel(),
    // Read through meta: the table keeps the first function it is given per column.
    meta: { facetCounts: facetCounts(data?.counts) },
    getFacetedUniqueValues: (table, columnId) => () =>
      new Map(Object.entries(table.options.meta?.facetCounts?.[columnId] ?? {})),
  })

  useEffect(() => {
    if (data) ensurePageInRange(pageCount)
  }, [data, pageCount, ensurePageInRange])

  // A new page, search, filter or order starts with nothing selected.
  useEffect(() => setRowSelection({}), [path])

  if (error) return <QueryError message={error.message} onRetry={() => void refetch()} />

  const filtered = Boolean(query) || columnFilters.length > 0
  return (
    <div className={cn('max-sm:has-[div[role="toolbar"]]:mb-16', 'flex flex-1 flex-col gap-4')}>
      <DataTableToolbar
        table={table}
        searchPlaceholder='按邮箱、名称或 ID 搜索…'
        filters={[
          { columnId: 'status', title: '状态', options: [...userStatuses] },
          { columnId: 'role', title: '角色', options: [...roles] },
        ]}
      />
      <DataTableView
        table={table}
        loading={isPending}
        emptyText={filtered ? '没有符合条件的用户。' : '还没有注册用户。'}
      />
      <p className='text-sm text-nowrap text-muted-foreground tabular-nums'>
        共 {data?.total.toLocaleString('zh-CN') ?? '…'} 个用户
      </p>
      <DataTablePagination table={table} className='mt-auto' />
      <DataTableBulkActions table={table} />
    </div>
  )
}
