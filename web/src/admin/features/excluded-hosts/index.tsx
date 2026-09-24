import { useEffect, useState } from 'react'
import {
  type ColumnDef,
  type SortingState,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { Ban, LogOut, Undo2 } from 'lucide-react'
import { toast } from 'sonner'
import { toastError, useAdminMutation, useAdminQuery } from '@/admin/lib/api'
import { formatDate, formatDateTime } from '@/admin/lib/format'
import { useTableUrlState } from '@/admin/hooks/use-table-url-state'
import { useNavigate, useSearch } from '@/admin/router'
import { Button } from '@/admin/components/ui/button'
import { ConfirmDialog } from '@/admin/components/confirm-dialog'
import {
  DataTableColumnHeader,
  DataTablePagination,
  DataTableToolbar,
  DataTableView,
} from '@/admin/components/data-table'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { LongText } from '@/admin/components/long-text'
import { QueryError } from '@/admin/components/query-error'
import type { ExcludedHost } from '@/lib/admin-types'

const reasons = [
  { label: '申请退出', value: 'opt_out' as const, icon: LogOut },
  { label: '永久封禁', value: 'blocked' as const, icon: Ban },
]

export function ExcludedHosts() {
  const [restoring, setRestoring] = useState<string | null>(null)
  const { data, isPending, error, refetch } = useAdminQuery<{ data: ExcludedHost[] }>('/excluded-hosts')
  const restore = useAdminMutation((host: string) => ({
    path: `/excluded-hosts/${encodeURIComponent(host)}`,
    method: 'DELETE',
  }))

  async function confirmRestore() {
    if (!restoring) return
    try {
      await restore.mutateAsync(restoring)
      toast.success(`${restoring} 已移出排除名单`)
      setRestoring(null)
    } catch (cause) {
      toastError(cause)
    }
  }

  const columns: ColumnDef<ExcludedHost>[] = [
    {
      accessorKey: 'host',
      header: ({ column }) => <DataTableColumnHeader column={column} title='域名' />,
      cell: ({ row }) => <span className='font-mono'>{row.original.host}</span>,
      enableHiding: false,
      meta: { title: '域名' },
    },
    {
      accessorKey: 'reason',
      header: ({ column }) => <DataTableColumnHeader column={column} title='原因' />,
      cell: ({ row }) => {
        const reason = reasons.find((item) => item.value === row.getValue('reason'))
        if (!reason) return null
        return (
          <div className='flex items-center gap-2'>
            <reason.icon className='size-4 text-muted-foreground' />
            <span>{reason.label}</span>
          </div>
        )
      },
      filterFn: (row, id, value) => value.includes(row.getValue(id)),
      meta: { title: '原因' },
    },
    {
      accessorKey: 'note',
      header: ({ column }) => <DataTableColumnHeader column={column} title='说明' />,
      cell: ({ row }) => (
        <LongText className='max-w-80 text-muted-foreground'>{row.original.note || '—'}</LongText>
      ),
      enableSorting: false,
      meta: { title: '说明', className: 'max-w-0 w-1/2 min-w-40' },
    },
    {
      accessorKey: 'created_at',
      header: ({ column }) => <DataTableColumnHeader column={column} title='加入时间' />,
      cell: ({ row }) => (
        <span className='text-nowrap text-muted-foreground' title={formatDateTime(row.original.created_at)}>
          {formatDate(row.original.created_at)}
        </span>
      ),
      meta: { title: '加入时间' },
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <Button variant='ghost' size='sm' className='h-8' onClick={() => setRestoring(row.original.host)}>
          <Undo2 />
          解除
        </Button>
      ),
      meta: { className: 'text-end' },
    },
  ]

  return (
    <>
      <AppHeader />
      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <PageTitle
          title='排除名单'
          description='申请退出或被封禁的域名，不能再被提交或收录。移除博客时可以把域名加进来。'
        />
        {error ? (
          <QueryError message={error.message} onRetry={() => void refetch()} />
        ) : (
          <ExcludedTable data={data?.data ?? []} loading={isPending} columns={columns} />
        )}
      </Main>
      <ConfirmDialog
        open={restoring !== null}
        onOpenChange={(open) => !open && !restore.isPending && setRestoring(null)}
        title={`解除 ${restoring ?? ''} 的排除？`}
        desc='解除后这个域名可以重新提交收录。'
        confirmText='解除'
        isLoading={restore.isPending}
        handleConfirm={() => void confirmRestore()}
        className='sm:max-w-md'
      />
    </>
  )
}

function ExcludedTable({
  data,
  loading,
  columns,
}: {
  data: ExcludedHost[]
  loading: boolean
  columns: ColumnDef<ExcludedHost>[]
}) {
  const [sorting, setSorting] = useState<SortingState>([{ id: 'created_at', desc: true }])
  const { globalFilter, onGlobalFilterChange, columnFilters, onColumnFiltersChange, pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: useSearch(),
      navigate: useNavigate(),
      pagination: { defaultPage: 1, defaultPageSize: 20 },
      globalFilter: { enabled: true, key: 'filter' },
      columnFilters: [{ columnId: 'reason', searchKey: 'reason', type: 'array' }],
    })

  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnFilters, globalFilter, pagination },
    getRowId: (item) => item.host,
    autoResetPageIndex: false,
    onSortingChange: setSorting,
    globalFilterFn: (row, _columnId, filterValue) =>
      row.original.host.includes(String(filterValue).toLowerCase()),
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
        searchPlaceholder='按域名筛选…'
        filters={[{ columnId: 'reason', title: '原因', options: reasons }]}
      />
      <DataTableView table={table} loading={loading} emptyText='排除名单是空的，所有域名都可以正常提交。' />
      <DataTablePagination table={table} className='mt-auto' />
    </div>
  )
}
