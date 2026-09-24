import { useAdminQuery } from '@/admin/lib/api'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { QueryError } from '@/admin/components/query-error'
import type { AdminSubmission } from '@/lib/admin-types'
import { SubmissionsDialogs } from './components/submissions-dialogs'
import { SubmissionsProvider } from './components/submissions-provider'
import { SubmissionsTable } from './components/submissions-table'

type List = { data: AdminSubmission[] }

export function Submissions() {
  // The API lists one status at a time; the table filters all three together.
  const pending = useAdminQuery<List>('/submissions?status=pending')
  const approved = useAdminQuery<List>('/submissions?status=approved')
  const rejected = useAdminQuery<List>('/submissions?status=rejected')
  const queries = [pending, approved, rejected]
  const failed = queries.find((query) => query.error)
  const data = queries.flatMap((query) => query.data?.data ?? [])

  return (
    <SubmissionsProvider>
      <AppHeader />
      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <PageTitle
          title='收录审核'
          description='读者提交的博客。通过后立即开始抓取，驳回原因会告诉提交者。'
        />
        {failed?.error ? (
          <QueryError message={failed.error.message} onRetry={() => queries.forEach((query) => void query.refetch())} />
        ) : (
          <SubmissionsTable data={data} loading={queries.some((query) => query.isPending)} />
        )}
      </Main>
      <SubmissionsDialogs />
    </SubmissionsProvider>
  )
}
