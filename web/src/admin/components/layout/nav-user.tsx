import { ChevronsUpDown, ExternalLink, LogOut, ShieldCheck, UserRound } from 'lucide-react'
import { Link } from '@/admin/router'
import { useAuth } from '@/admin/context/auth-provider'
import useDialogState from '@/admin/hooks/use-dialog-state'
import { Badge } from '@/admin/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/admin/components/ui/dropdown-menu'
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from '@/admin/components/ui/sidebar'
import { SignOutDialog } from '@/admin/components/sign-out-dialog'
import { UserAvatar } from '@/admin/components/user-avatar'

/** The signed-in admin at the foot of the sidebar, the one place for account actions. */
export function NavUser() {
  const { isMobile, setOpenMobile } = useSidebar()
  const { user } = useAuth()
  const [open, setOpen] = useDialogState()
  if (!user) return null

  return (
    <>
      <SidebarMenu>
        <SidebarMenuItem>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <SidebarMenuButton
                size='lg'
                className='data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground'
              >
                <UserAvatar name={user.name} email={user.email} />
                <div className='grid flex-1 text-start text-sm leading-tight'>
                  <span className='truncate font-medium'>{user.name}</span>
                  <span className='truncate text-xs text-muted-foreground'>{user.email}</span>
                </div>
                <ChevronsUpDown className='ms-auto size-4 text-muted-foreground' />
              </SidebarMenuButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              className='w-(--radix-dropdown-menu-trigger-width) min-w-64 rounded-lg'
              side={isMobile ? 'bottom' : 'right'}
              align='end'
              sideOffset={4}
            >
              <DropdownMenuLabel className='p-0 font-normal'>
                <div className='flex items-center gap-3 p-2 text-start'>
                  <UserAvatar name={user.name} email={user.email} className='size-10 text-base' />
                  <div className='grid min-w-0 flex-1 gap-1 leading-tight'>
                    <div className='flex min-w-0 items-center gap-2'>
                      <span className='truncate text-sm font-semibold'>{user.name}</span>
                      <Badge variant='secondary'>
                        <ShieldCheck />
                        管理员
                      </Badge>
                    </div>
                    <span className='truncate text-xs text-muted-foreground'>{user.email}</span>
                  </div>
                </div>
              </DropdownMenuLabel>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuItem asChild>
                  <Link to='/admin/profile' onClick={() => setOpenMobile(false)}>
                    <UserRound />
                    个人资料
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <a href='/' target='_blank' rel='noopener'>
                    <ExternalLink />
                    查看前台
                  </a>
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant='destructive' onClick={() => setOpen(true)}>
                <LogOut />
                退出登录
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </SidebarMenuItem>
      </SidebarMenu>

      <SignOutDialog open={!!open} onOpenChange={setOpen} />
    </>
  )
}
