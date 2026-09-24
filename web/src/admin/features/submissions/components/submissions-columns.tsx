import { type ColumnDef } from '@tanstack/react-table'
import { cn } from '@/admin/lib/utils'
import { formatDateTime } from '@/admin/lib/format'
import { BlogAvatar } from '@/admin/components/blog-avatar'
import { DataTableColumnHeader } from '@/admin/components/data-table'
import { problemCounts } from '@/admin/components/check-problems'
import { TimeAgo } from '@/admin/components/time-ago'
import type { AdminSubmission } from '@/lib/admin-types'
import { checkResults, statusOrder, submissionStatuses } from '../data/data'
import { DataTableRowActions } from './data-table-row-actions'
import { useSubmissions } from './submissions-provider'

function SubmissionCell({ submission }: { submission: AdminSubmission }) {
  const { setOpen, setCurrentRow } = useSubmissions()
  const title = submission.check_report?.title
  return (
    <button
      type='button'
      className='flex w-full items-center gap-3 text-start'
      onClick={() => {
        setCurrentRow(submission)
        setOpen('detail')
      }}
    >
      <BlogAvatar host={submission.host} name={title || submission.host} />
      <span className='grid min-w-0'>
        <span className='truncate font-medium hover:underline'>{title || submission.host}</span>
        <span className='truncate font-mono text-xs text-muted-foreground'>{submission.host}</span>
      </span>
    </button>
  )
}

export const submissionsColumns: ColumnDef<AdminSubmission>[] = [
  {
    id: 'blog',
    accessorFn: (submission) => submission.host,
    header: ({ column }) => <DataTableColumnHeader column={column} title='博客' />,
    cell: ({ row }) => <SubmissionCell submission={row.original} />,
    enableHiding: false,
    meta: { title: '博客', className: 'w-1/3 max-w-0 min-w-56' },
  },
  {
    id: 'check',
    accessorFn: (submission) => (submission.check_report?.passed ? 'passed' : 'failed'),
    header: ({ column }) => <DataTableColumnHeader column={column} title='检查' />,
    cell: ({ row }) => {
      const report = row.original.check_report
      const result = checkResults.find((item) => item.value === row.getValue('check'))
      if (!result) return null
      const { errors, warnings } = problemCounts(report?.problems)
      return (
        <div className={cn('flex items-center gap-2', result.className)}>
          <result.icon className='size-4' />
          <span className='text-nowrap'>
            {result.label}
            {(errors > 0 || warnings > 0) && (
              <span className='ms-1 text-muted-foreground'>
                （{[errors && `${errors} 错误`, warnings && `${warnings} 警告`].filter(Boolean).join('，')}）
              </span>
            )}
          </span>
        </div>
      )
    },
    filterFn: (row, id, value) => value.includes(row.getValue(id)),
    meta: { title: '检查' },
  },
  {
    id: 'articles',
    accessorFn: (submission) => submission.check_report?.items?.valid ?? 0,
    header: ({ column }) => <DataTableColumnHeader column={column} title='有效文章' />,
    cell: ({ row }) => <span className='tabular-nums text-muted-foreground'>{row.getValue('articles')}</span>,
    meta: { title: '有效文章' },
  },
  {
    accessorKey: 'status',
    header: ({ column }) => <DataTableColumnHeader column={column} title='状态' />,
    cell: ({ row }) => {
      const status = submissionStatuses.find((item) => item.value === row.getValue('status'))
      if (!status) return null
      return (
        <div className={cn('flex w-24 items-center gap-2', status.className)}>
          <status.icon className='size-4' />
          <span>{status.label}</span>
        </div>
      )
    },
    sortingFn: (a, b) => statusOrder[a.original.status] - statusOrder[b.original.status],
    filterFn: (row, id, value) => value.includes(row.getValue(id)),
    meta: { title: '状态' },
  },
  {
    accessorKey: 'created_at',
    header: ({ column }) => <DataTableColumnHeader column={column} title='提交时间' />,
    cell: ({ row }) => (
      <TimeAgo
        iso={row.original.created_at}
        className='text-nowrap text-muted-foreground'
      />
    ),
    meta: { title: '提交时间' },
  },
  {
    id: 'reviewed_at',
    accessorFn: (submission) => submission.reviewed_at ?? '',
    header: ({ column }) => <DataTableColumnHeader column={column} title='处理时间' />,
    cell: ({ row }) => (
      <span className='text-nowrap text-muted-foreground'>{formatDateTime(row.original.reviewed_at)}</span>
    ),
    meta: { title: '处理时间' },
  },
  {
    id: 'actions',
    cell: ({ row }) => <DataTableRowActions row={row} />,
  },
]
