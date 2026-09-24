import { useEffect, useState } from 'react'
import { type VisibilityState, getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { useAdminQuery } from '@/admin/lib/api'
import { useDebounced } from '@/admin/hooks/use-debounced'
import { useTableUrlState } from '@/admin/hooks/use-table-url-state'
import { useNavigate, useSearch } from '@/admin/router'
import { DataTablePagination, DataTableToolbar, DataTableView } from '@/admin/components/data-table'
import { QueryError } from '@/admin/components/query-error'
import type { AdminUserRow, Paged } from '@/lib/admin-types'
import { usersColumns as columns } from './users-columns'

/** Users are paged and searched by the API, so the table only shows one page. */
export function UsersTable() {
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})
  const { globalFilter, onGlobalFilterChange, pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: useSearch(),
      navigate: useNavigate(),
      pagination: { defaultPage: 1, defaultPageSize: 20 },
      globalFilter: { enabled: true, key: 'filter' },
    })
  const query = useDebounced(globalFilter ?? '')
  const path = `/users?limit=${pagination.pageSize}&offset=${pagination.pageIndex * pagination.pageSize}&q=${encodeURIComponent(query)}`
  const { data, isPending, error, refetch } = useAdminQuery<Paged<AdminUserRow>>(path, { keepPrevious: true })
  const pageCount = Math.max(1, Math.ceil((data?.total ?? 0) / pagination.pageSize))

  const table = useReactTable({
    data: data?.data ?? [],
    columns,
    state: { columnVisibility, globalFilter, pagination },
    getRowId: (user) => user.id,
    manualPagination: true,
    manualFiltering: true,
    pageCount,
    onColumnVisibilityChange: setColumnVisibility,
    onPaginationChange,
    onGlobalFilterChange,
    getCoreRowModel: getCoreRowModel(),
  })

  useEffect(() => {
    if (data) ensurePageInRange(pageCount)
  }, [data, pageCount, ensurePageInRange])

  if (error) return <QueryError message={error.message} onRetry={() => void refetch()} />

  return (
    <div className='flex flex-1 flex-col gap-4'>
      <DataTableToolbar table={table} searchPlaceholder='按邮箱或名称搜索…' />
      <DataTableView
        table={table}
        loading={isPending}
        emptyText={query ? `没有邮箱或名称包含“${query}”的用户。` : '还没有注册用户。'}
      />
      <DataTablePagination table={table} className='mt-auto' />
    </div>
  )
}
