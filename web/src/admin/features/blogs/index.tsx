import { Plus } from 'lucide-react'
import { Button } from '@/admin/components/ui/button'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { QueryError } from '@/admin/components/query-error'
import { useBlogsQuery } from './api'
import { BlogsDialogs } from './components/blogs-dialogs'
import { BlogsProvider, useBlogs } from './components/blogs-provider'
import { BlogsTable } from './components/blogs-table'

export function Blogs() {
  return (
    <BlogsProvider>
      <AppHeader />
      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <PageTitle title='博客管理' description='已收录的博客：编辑资料、暂停、立即抓取或移除。'>
          <BlogsPrimaryButtons />
        </PageTitle>
        <BlogsContent />
      </Main>
      <BlogsDialogs />
    </BlogsProvider>
  )
}

function BlogsPrimaryButtons() {
  const { setOpen } = useBlogs()
  return (
    <Button className='space-x-1' onClick={() => setOpen('create')}>
      <span>添加博客</span> <Plus size={18} />
    </Button>
  )
}

function BlogsContent() {
  const { data, isPending, error, refetch } = useBlogsQuery()
  if (error) return <QueryError message={error.message} onRetry={() => void refetch()} />
  return <BlogsTable data={data?.data ?? []} loading={isPending} />
}
