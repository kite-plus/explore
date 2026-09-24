import { lazy, Suspense, useEffect, useState, type ComponentType } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { createQueryClient } from '@/admin/lib/api'
import { AuthProvider, useAuth } from '@/admin/context/auth-provider'
import { ThemeProvider } from '@/admin/context/theme-provider'
import { RouterProvider, useLocation } from '@/admin/router'
import { adminTitle } from '@/admin/routes'
import { AuthenticatedLayout } from '@/admin/components/layout/authenticated-layout'
import { Toaster } from '@/admin/components/ui/sonner'
import { SignIn } from '@/admin/features/auth/sign-in'
import { NotFoundError } from '@/admin/features/errors/not-found-error'

// Each page is its own chunk, as in shadcn-admin's route-based splitting, so
// the charts and forms load only on the pages that use them.
const Dashboard = lazy(() => import('@/admin/features/dashboard').then((m) => ({ default: m.Dashboard })))
const Submissions = lazy(() => import('@/admin/features/submissions').then((m) => ({ default: m.Submissions })))
const Blogs = lazy(() => import('@/admin/features/blogs').then((m) => ({ default: m.Blogs })))
const Entries = lazy(() => import('@/admin/features/entries').then((m) => ({ default: m.Entries })))
const Takedowns = lazy(() => import('@/admin/features/takedowns').then((m) => ({ default: m.Takedowns })))
const Users = lazy(() => import('@/admin/features/users').then((m) => ({ default: m.Users })))
const Queue = lazy(() => import('@/admin/features/queue').then((m) => ({ default: m.Queue })))
const ExcludedHosts = lazy(() => import('@/admin/features/excluded-hosts').then((m) => ({ default: m.ExcludedHosts })))
const Tools = lazy(() => import('@/admin/features/tools').then((m) => ({ default: m.Tools })))
const Settings = lazy(() => import('@/admin/features/settings').then((m) => ({ default: m.Settings })))

const PAGES: Record<string, ComponentType> = {
  '/admin': Dashboard,
  '/admin/submissions': Submissions,
  '/admin/blogs': Blogs,
  '/admin/entries': Entries,
  '/admin/takedowns': Takedowns,
  '/admin/users': Users,
  '/admin/queue': Queue,
  '/admin/excluded-hosts': ExcludedHosts,
  '/admin/tools': Tools,
  '/admin/settings': Settings,
  '/admin/settings/notice': Settings,
  '/admin/settings/appearance': Settings,
}

export function AdminApp() {
  const [queryClient] = useState(createQueryClient)
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AuthProvider>
          <RouterProvider>
            {/* Mounted before the pages so toasts from their first effects are not missed. */}
            <Toaster duration={5000} />
            <AdminRoot />
          </RouterProvider>
        </AuthProvider>
      </ThemeProvider>
    </QueryClientProvider>
  )
}

function AdminRoot() {
  const { token, checking } = useAuth()
  const { pathname } = useLocation()

  useEffect(() => {
    document.title = adminTitle(pathname)
  }, [pathname])

  if (checking) {
    return (
      <div className='flex h-svh items-center justify-center gap-2 text-sm text-muted-foreground'>
        <Loader2 className='size-4 animate-spin' />
        正在连接管理后台…
      </div>
    )
  }
  if (token === null) return <SignIn />

  const Page = PAGES[pathname] ?? NotFoundError
  return (
    <AuthenticatedLayout>
      <Suspense
        fallback={
          <div className='flex flex-1 items-center justify-center gap-2 py-24 text-sm text-muted-foreground'>
            <Loader2 className='size-4 animate-spin' />
            正在加载…
          </div>
        }
      >
        <Page />
      </Suspense>
    </AuthenticatedLayout>
  )
}
