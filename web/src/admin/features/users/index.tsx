import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { PageTitle } from '@/admin/components/layout/page-title'
import { UsersDialogs } from './components/users-dialogs'
import { UsersProvider } from './components/users-provider'
import { UsersTable } from './components/users-table'

export function Users() {
  return (
    <UsersProvider>
      <AppHeader />
      <Main className='flex flex-1 flex-col gap-4 sm:gap-6'>
        <PageTitle title='用户管理' description='读者账号：订阅和博客认领，停用账号或调整后台权限。' />
        <UsersTable />
      </Main>
      <UsersDialogs />
    </UsersProvider>
  )
}
