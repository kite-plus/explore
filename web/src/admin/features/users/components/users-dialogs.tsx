import { toast } from 'sonner'
import { toastError, useAdminMutation } from '@/admin/lib/api'
import { ConfirmDialog } from '@/admin/components/confirm-dialog'
import { changeCopy, type UserChange } from '../data/data'
import { useUsers } from './users-provider'

export function UsersDialogs() {
  const { pending, setPending } = useUsers()
  const update = useAdminMutation(({ id, change }: { id: string; change: UserChange }) => ({
    path: `/users/${encodeURIComponent(id)}`,
    method: 'PATCH',
    body: { [change.field]: change.value },
  }))
  if (!pending) return null
  const copy = changeCopy(pending.change)

  async function confirm() {
    if (!pending) return
    try {
      await update.mutateAsync({ id: pending.user.id, change: pending.change })
      toast.success(`${pending.user.email}：${copy.title}完成`)
      setPending(null)
    } catch (cause) {
      toastError(cause)
    }
  }

  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => !open && !update.isPending && setPending(null)}
      title={copy.title}
      desc={
        <>
          <span className='font-medium text-foreground'>
            {pending.user.display_name}（{pending.user.email}）
          </span>
          <br />
          {copy.desc}
        </>
      }
      confirmText={copy.action}
      destructive={copy.destructive}
      isLoading={update.isPending}
      handleConfirm={() => void confirm()}
      className='sm:max-w-md'
    />
  )
}
