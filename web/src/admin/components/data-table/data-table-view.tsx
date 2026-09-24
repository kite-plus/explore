import { flexRender, type Table as TableInstance } from '@tanstack/react-table'
import { cn } from '@/admin/lib/utils'
import { Skeleton } from '@/admin/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/admin/components/ui/table'

type DataTableViewProps<TData> = {
  table: TableInstance<TData>
  loading?: boolean
  emptyText?: string
  className?: string
}

/** The table markup shared by every list, as in shadcn-admin's tasks table. */
export function DataTableView<TData>({
  table,
  loading = false,
  emptyText = '没有结果。',
  className,
}: DataTableViewProps<TData>) {
  const columnCount = table.getVisibleLeafColumns().length
  const rows = table.getRowModel().rows

  return (
    <div className='overflow-hidden rounded-md border'>
      <Table className={cn('min-w-xl', className)}>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <TableHead
                  key={header.id}
                  colSpan={header.colSpan}
                  className={cn(
                    header.column.columnDef.meta?.className,
                    header.column.columnDef.meta?.thClassName
                  )}
                >
                  {header.isPlaceholder
                    ? null
                    : flexRender(header.column.columnDef.header, header.getContext())}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {loading ? (
            Array.from({ length: 6 }, (_, index) => (
              <TableRow key={index}>
                <TableCell colSpan={columnCount}>
                  <Skeleton className='h-6 w-full' />
                </TableCell>
              </TableRow>
            ))
          ) : rows.length ? (
            rows.map((row) => (
              <TableRow key={row.id} data-state={row.getIsSelected() && 'selected'}>
                {row.getVisibleCells().map((cell) => (
                  <TableCell
                    key={cell.id}
                    className={cn(
                      cell.column.columnDef.meta?.className,
                      cell.column.columnDef.meta?.tdClassName
                    )}
                  >
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell colSpan={columnCount} className='h-24 text-center'>
                {emptyText}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}
