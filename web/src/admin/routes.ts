// Page titles by path. Shared by the Astro page (for the first <title>)
// and the app (for titles after client-side navigation).
export const ADMIN_TITLES: Record<string, string> = {
  '/admin': '工作台',
  '/admin/submissions': '收录审核',
  '/admin/blogs': '博客管理',
  '/admin/entries': '文章管理',
  '/admin/takedowns': '下架审批',
  '/admin/users': '用户管理',
  '/admin/queue': '抓取任务',
  '/admin/excluded-hosts': '排除名单',
  '/admin/tools': '诊断工具',
  '/admin/settings': '系统设置',
  '/admin/settings/notice': '站点公告',
  '/admin/settings/appearance': '外观',
}

export function adminTitle(pathname: string) {
  const path = pathname.length > 1 ? pathname.replace(/\/+$/, '') : pathname
  return `${ADMIN_TITLES[path] ?? '页面不存在'} · Explore 管理后台`
}
