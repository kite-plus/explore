import { Megaphone, Palette, SlidersHorizontal } from 'lucide-react'
import { useLocation } from '@/admin/router'
import { Separator } from '@/admin/components/ui/separator'
import { AppHeader } from '@/admin/components/layout/app-header'
import { Main } from '@/admin/components/layout/main'
import { AppearanceForm } from './appearance/appearance-form'
import { ContentSection } from './components/content-section'
import { SidebarNav } from './components/sidebar-nav'
import { GeneralForm } from './general/general-form'
import { NoticeForm } from './notice/notice-form'

const sidebarNavItems = [
  { title: '常规', href: '/admin/settings', icon: <SlidersHorizontal size={18} /> },
  { title: '站点公告', href: '/admin/settings/notice', icon: <Megaphone size={18} /> },
  { title: '外观', href: '/admin/settings/appearance', icon: <Palette size={18} /> },
]

export function Settings() {
  const { pathname } = useLocation()

  return (
    <>
      <AppHeader fixed={false} />

      <Main fixed>
        <div className='space-y-0.5'>
          <h1 className='text-2xl font-bold tracking-tight md:text-3xl'>系统设置</h1>
          <p className='text-muted-foreground'>
            保存在数据库中的站点开关和公告，以及这台设备上的后台外观。
          </p>
        </div>
        <Separator className='my-4 lg:my-6' />
        <div className='flex flex-1 flex-col space-y-2 overflow-hidden md:space-y-2 lg:flex-row lg:space-y-0 lg:space-x-12'>
          <aside className='top-0 lg:sticky lg:w-1/5'>
            <SidebarNav items={sidebarNavItems} />
          </aside>
          <div className='flex w-full overflow-y-hidden p-1'>
            {pathname === '/admin/settings/notice' ? (
              <ContentSection title='站点公告' desc='显示在前台每个页面的导航下方，留空则不显示。'>
                <NoticeForm />
              </ContentSection>
            ) : pathname === '/admin/settings/appearance' ? (
              <ContentSection title='外观' desc='后台的配色，只保存在这台设备上，和前台共用。'>
                <AppearanceForm />
              </ContentSection>
            ) : (
              <ContentSection title='常规' desc='这些开关立即影响注册、投稿和抓取流程。'>
                <GeneralForm />
              </ContentSection>
            )}
          </div>
        </div>
      </Main>
    </>
  )
}
