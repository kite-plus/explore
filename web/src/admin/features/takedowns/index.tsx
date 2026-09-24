import { Plus } from 'lucide-react'
import { useAdminQuery } from '@/admin/lib/api'
import { Button } from '@/admin/components/ui/button'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { QueryError } from '@/admin/components/query-error'
import type { Takedown } from '@/lib/admin-types'
import { TakedownsDialogs } from './components/takedowns-dialogs'
import { TakedownsProvider, useTakedowns } from './components/takedowns-provider'
import { TakedownsTable } from './components/takedowns-table'

type List = { data: Takedown[] }

export function Takedowns() {
  return (
    <TakedownsProvider>
      <AppHeader />
      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <PageTitle title='下架审批' description='读者举报和内部申请。通过后暂停博客或隐藏文章。'>
          <CreateButton />
        </PageTitle>
        <TakedownsContent />
      </Main>
      <TakedownsDialogs />
    </TakedownsProvider>
  )
}

function CreateButton() {
  const { setOpen } = useTakedowns()
  return (
    <Button className='space-x-1' onClick={() => setOpen('create')}>
      <span>新建申请</span> <Plus size={18} />
    </Button>
  )
}

function TakedownsContent() {
  // The API lists one status at a time; the table filters all three together.
  const pending = useAdminQuery<List>('/takedowns?status=pending')
  const approved = useAdminQuery<List>('/takedowns?status=approved')
  const rejected = useAdminQuery<List>('/takedowns?status=rejected')
  const queries = [pending, approved, rejected]
  const failed = queries.find((query) => query.error)
  if (failed?.error)
    return <QueryError message={failed.error.message} onRetry={() => queries.forEach((query) => void query.refetch())} />
  return (
    <TakedownsTable
      data={queries.flatMap((query) => query.data?.data ?? [])}
      loading={queries.some((query) => query.isPending)}
    />
  )
}
