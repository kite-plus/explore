import { useAuth } from '@/admin/context/auth-provider'
import { ConfirmDialog } from '@/admin/components/confirm-dialog'

interface SignOutDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SignOutDialog({ open, onOpenChange }: SignOutDialogProps) {
  const { logout } = useAuth()

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title='退出登录'
      desc='确定要退出吗？之后需要重新登录才能进入后台。'
      confirmText='退出登录'
      destructive
      handleConfirm={() => {
        onOpenChange(false)
        logout()
      }}
      className='sm:max-w-sm'
    />
  )
}
