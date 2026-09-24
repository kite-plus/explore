import { ExternalLink, LogOut, Settings } from 'lucide-react'
import { Link } from '@/admin/router'
import { getDisplayNameInitials } from '@/admin/lib/utils'
import { useAuth } from '@/admin/context/auth-provider'
import useDialogState from '@/admin/hooks/use-dialog-state'
import { Avatar, AvatarFallback } from '@/admin/components/ui/avatar'
import { Button } from '@/admin/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/admin/components/ui/dropdown-menu'
import { SignOutDialog } from '@/admin/components/sign-out-dialog'

export function ProfileDropdown() {
  const [open, setOpen] = useDialogState()
  const { user } = useAuth()

  return (
    <>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger asChild>
          <Button variant='ghost' className='relative h-8 w-8 rounded-full'>
            <Avatar className='h-8 w-8'>
              <AvatarFallback>
                {getDisplayNameInitials(user?.name ?? '')}
              </AvatarFallback>
            </Avatar>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent className='w-56' align='end' forceMount>
          <DropdownMenuLabel className='font-normal'>
            <div className='flex flex-col gap-1.5'>
              <p className='text-sm leading-none font-medium'>
                {user?.name ?? '管理员'}
              </p>
              <p className='text-xs leading-none text-muted-foreground'>
                {user?.email}
              </p>
            </div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem asChild>
              <Link to='/admin/settings'>
                系统设置
                <DropdownMenuShortcut>
                  <Settings size={16} />
                </DropdownMenuShortcut>
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <a href='/' target='_blank' rel='noopener'>
                查看前台
                <DropdownMenuShortcut>
                  <ExternalLink size={16} />
                </DropdownMenuShortcut>
              </a>
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant='destructive' onClick={() => setOpen(true)}>
            退出登录
            <DropdownMenuShortcut className='text-current'>
              <LogOut size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <SignOutDialog open={!!open} onOpenChange={setOpen} />
    </>
  )
}
