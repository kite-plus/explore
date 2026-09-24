import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { EntriesProvider } from './components/entries-provider'
import { EntriesTable } from './components/entries-table'

export function Entries() {
  return (
    <EntriesProvider>
      <AppHeader />
      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <PageTitle
          title='文章管理'
          description='抓取缓存中的文章。隐藏后在前台消失，重新抓取也不会恢复。'
        />
        <EntriesTable />
      </Main>
    </EntriesProvider>
  )
}
