import { useLayout } from '@/admin/context/layout-provider'
import { useAdminQuery } from '@/admin/lib/api'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarRail,
} from '@/admin/components/ui/sidebar'
import type { AdminOverview } from '@/lib/admin-types'
import { AppTitle } from './app-title'
import { sidebarData } from './data/sidebar-data'
import { NavGroup } from './nav-group'
import { NavUser } from './nav-user'

export function AppSidebar() {
  const { collapsible, variant } = useLayout()
  const { data: overview } = useAdminQuery<AdminOverview>('/overview')
  const badges: Record<string, number | undefined> = {
    '/admin/submissions': overview?.stats.pending_submissions,
    '/admin/takedowns': overview?.stats.pending_takedowns,
  }

  return (
    <Sidebar collapsible={collapsible} variant={variant}>
      <SidebarHeader>
        <AppTitle />
      </SidebarHeader>
      <SidebarContent>
        {sidebarData.navGroups.map((group) => (
          <NavGroup
            key={group.title}
            title={group.title}
            items={group.items.map((item) =>
              item.url && badges[item.url]
                ? { ...item, badge: String(badges[item.url]) }
                : item
            )}
          />
        ))}
      </SidebarContent>
      <SidebarFooter>
        <NavUser />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
