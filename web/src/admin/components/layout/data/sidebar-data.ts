import {
  Activity,
  Ban,
  BookOpen,
  FileText,
  Flag,
  Inbox,
  LayoutDashboard,
  Megaphone,
  Palette,
  Settings,
  SlidersHorizontal,
  Stethoscope,
  Users,
} from 'lucide-react'
import { type SidebarData } from '../types'

export const sidebarData: SidebarData = {
  navGroups: [
    {
      title: '概览',
      items: [{ title: '工作台', url: '/admin', icon: LayoutDashboard }],
    },
    {
      title: '内容',
      items: [
        { title: '收录审核', url: '/admin/submissions', icon: Inbox },
        { title: '博客管理', url: '/admin/blogs', icon: BookOpen },
        { title: '文章管理', url: '/admin/entries', icon: FileText },
        { title: '下架审批', url: '/admin/takedowns', icon: Flag },
      ],
    },
    {
      title: '运营',
      items: [
        { title: '用户管理', url: '/admin/users', icon: Users },
        { title: '抓取任务', url: '/admin/queue', icon: Activity },
      ],
    },
    {
      title: '系统',
      items: [
        {
          title: '系统设置',
          icon: Settings,
          items: [
            { title: '常规', url: '/admin/settings', icon: SlidersHorizontal },
            { title: '站点公告', url: '/admin/settings/notice', icon: Megaphone },
            { title: '外观', url: '/admin/settings/appearance', icon: Palette },
          ],
        },
        { title: '排除名单', url: '/admin/excluded-hosts', icon: Ban },
        { title: '诊断工具', url: '/admin/tools', icon: Stethoscope },
      ],
    },
  ],
}
