import { Search } from '@/admin/components/search'
import { ThemeSwitch } from '@/admin/components/theme-switch'
import { Header } from './header'

/** The header every page uses, as shadcn-admin's pages do. */
export function AppHeader({ fixed = true }: { fixed?: boolean }) {
  return (
    <Header fixed={fixed}>
      <Search className='me-auto' />
      <ThemeSwitch />
    </Header>
  )
}
