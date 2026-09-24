import { Link } from '@/admin/router'
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from '@/admin/components/ui/sidebar'

export function AppTitle() {
  const { setOpenMobile } = useSidebar()
  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <SidebarMenuButton size='lg' asChild>
          <Link to='/admin' onClick={() => setOpenMobile(false)}>
            <div className='flex aspect-square size-8 items-center justify-center rounded-lg border bg-background'>
              <img src='/favicon.svg' alt='' width={20} height={17} />
            </div>
            <div className='grid flex-1 text-start text-sm leading-tight'>
              <span className='truncate font-semibold'>Explore</span>
              <span className='truncate text-xs'>管理后台</span>
            </div>
          </Link>
        </SidebarMenuButton>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
